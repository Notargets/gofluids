package model_problems

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/notargets/avs/chart2d"
	utils2 "github.com/notargets/avs/utils"

	"github.com/notargets/gocfd/DG1D"
	"github.com/notargets/gocfd/utils"
)

type MaxwellDFR struct {
	// Input parameters
	CFL, FinalTime                   float64
	El                               *DG1D.Elements1D
	RHSOnce, PlotOnce                sync.Once
	E, H                             utils.Matrix
	Epsilon, Mu                      utils.Matrix
	Zimp, ZimPM, ZimPP, YimPM, YimPP utils.Matrix
	chart                            *chart2d.Chart2D
	colorMap                         *utils2.ColorMap
}

func NewMaxwellDFR(CFL, FinalTime float64, N, K int) (c *MaxwellDFR) {
	VX, EToV := DG1D.SimpleMesh1D(-2, 2, K)
	c = &MaxwellDFR{
		CFL:       CFL,
		FinalTime: FinalTime,
		El:        DG1D.NewElements1D(N, VX, EToV),
	}
	fmt.Printf("CFL = %8.4f, Polynomial Degree N = %d (1 is linear), Num Elements K = %d\n\n\n", CFL, N, K)
	epsData := utils.ConstArray(c.El.K, 1)
	ones := utils.NewVectorConstant(c.El.Np, 1)
	for i := c.El.K / 2; i < c.El.K; i++ {
		epsData[i] = 2
	}
	Eps1 := utils.NewVector(c.El.K, epsData)
	c.Epsilon = Eps1.Outer(ones)
	Mu1 := utils.NewVectorConstant(c.El.K, 1)
	c.Mu = Mu1.Outer(ones)
	c.E = c.El.X.Copy().Apply(func(val float64) float64 {
		if val < 0 {
			return math.Sin(math.Pi * val)
		} else {
			return 0
		}
	})
	c.H = utils.NewMatrix(c.El.Np, c.El.K)
	c.Zimp = c.Epsilon.Copy().POW(-1).ElMul(c.Mu).Apply(math.Sqrt)
	nrF, ncF := c.El.Nfp*c.El.NFaces, c.El.K
	c.ZimPM = c.Zimp.Subset(c.El.VmapM, nrF, ncF)
	c.ZimPP = c.Zimp.Subset(c.El.VmapP, nrF, ncF)
	c.ZimPM.SetReadOnly("ZimPM")
	c.ZimPP.SetReadOnly("ZimPP")
	c.YimPM, c.YimPP = c.ZimPM.Copy().POW(-1), c.ZimPP.Copy().POW(-1)
	c.YimPM.SetReadOnly("YimPM")
	c.YimPP.SetReadOnly("YimPP")
	return
}

func (c *MaxwellDFR) Run(showGraph bool, graphDelay ...time.Duration) {
	var (
		el           = c.El
		resE         = utils.NewMatrix(el.Np, el.K)
		resH         = utils.NewMatrix(el.Np, el.K)
		logFrequency = 50
		limiter      = false
		limiterM     = 20.
	)
	xmin := el.X.Row(1).Subtract(el.X.Row(0)).Apply(math.Abs).Min()
	dt := xmin * c.CFL
	Nsteps := int(math.Ceil(c.FinalTime / dt))
	dt = c.FinalTime / float64(Nsteps)
	fmt.Printf("FinalTime = %8.4f, Nsteps = %d, dt = %8.6f\n", c.FinalTime, Nsteps, dt)

	var Time float64
	for tstep := 0; tstep < Nsteps; tstep++ {
		c.Plot(showGraph, graphDelay, c.E, c.H)
		for INTRK := 0; INTRK < 5; INTRK++ {
			if limiter {
				c.E = el.SlopeLimitN(c.E, limiterM)
				c.H = el.SlopeLimitN(c.H, limiterM)
			}
			rhsE, rhsH := c.RHS()
			resE.Scale(utils.RK4a[INTRK]).Add(rhsE.Scale(dt))
			resH.Scale(utils.RK4a[INTRK]).Add(rhsH.Scale(dt))
			c.E.Add(resE.Copy().Scale(utils.RK4b[INTRK]))
			c.H.Add(resH.Copy().Scale(utils.RK4b[INTRK]))
		}
		Time += dt
		if tstep%logFrequency == 0 {
			fmt.Printf("Time = %8.4f, max_resid[%d] = %8.4f, emin = %8.6f, emax = %8.6f\n", Time, tstep, resE.Max(), c.E.Min(), c.E.Max())
		}
	}
	return
}

func (c *MaxwellDFR) RHS() (RHSE, RHSH utils.Matrix) {
	var (
		nrF, ncF = c.El.Nfp * c.El.NFaces, c.El.K
		// Field flux differerence across faces
		dE           = c.E.Subset(c.El.VmapM, nrF, ncF).Subtract(c.E.Subset(c.El.VmapP, nrF, ncF))
		dH           = c.H.Subset(c.El.VmapM, nrF, ncF).Subtract(c.H.Subset(c.El.VmapP, nrF, ncF))
		el           = c.El
		fluxE, fluxH utils.Matrix
	)
	// Homogeneous boundary conditions at the inflow faces, Ez = 0
	// Reflection BC - Metal boundary - E is zero at shell face, H passes through (Neumann)
	// E on the boundary face is negative of E inside, so the diff in E at the boundary face is 2E of the interior
	dE.AssignVector(el.MapB, c.E.SubsetVector(el.VmapB).Scale(2))
	// H on the boundary face is equal to H inside, so the diff in H at the boundary face is 0
	dH.AssignVector(el.MapB, c.H.SubsetVector(el.VmapB).Set(0))

	// Upwind fluxes
	fluxE = c.ZimPM.Copy().Add(c.ZimPP).POW(-1).ElMul(el.NX.Copy().ElMul(c.ZimPP).ElMul(dH).Subtract(dE))
	fluxH = c.YimPM.Copy().Add(c.YimPP).POW(-1).ElMul(el.NX.Copy().ElMul(c.YimPP).ElMul(dE).Subtract(dH))

	RHSE = el.Rx.Copy().Scale(-1).ElMul(el.Dr.Mul(c.H)).Add(el.LIFT.Mul(fluxE.ElMul(el.FScale))).ElDiv(c.Epsilon)
	RHSH = el.Rx.Copy().Scale(-1).ElMul(el.Dr.Mul(c.E)).Add(el.LIFT.Mul(fluxH.ElMul(el.FScale))).ElDiv(c.Mu)

	return
}

func (c *MaxwellDFR) Plot(showGraph bool, graphDelay []time.Duration, E, H utils.Matrix) {
	var (
		el         = c.El
		pMin, pMax = float32(-1), float32(1)
	)
	if !showGraph {
		return
	}
	c.PlotOnce.Do(func() {
		c.chart = chart2d.NewChart2D(1280, 1024, float32(el.X.Min()), float32(el.X.Max()), pMin, pMax)
		c.colorMap = utils2.NewColorMap(-1, 1, 1)
		go c.chart.Plot()
	})

	if err := c.chart.AddSeries("E", el.X.Transpose().RawMatrix().Data, E.Transpose().RawMatrix().Data,
		chart2d.NoGlyph, chart2d.Solid, c.colorMap.GetRGB(0)); err != nil {
		panic("unable to add graph series")
	}
	if err := c.chart.AddSeries("H", el.X.Transpose().RawMatrix().Data, H.Transpose().RawMatrix().Data,
		chart2d.NoGlyph, chart2d.Solid, c.colorMap.GetRGB(0.7)); err != nil {
		panic("unable to add graph series")
	}
	if len(graphDelay) != 0 {
		time.Sleep(graphDelay[0])
	}
}
