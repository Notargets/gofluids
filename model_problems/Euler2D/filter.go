package Euler2D

import (
	"fmt"
	"math"

	"github.com/notargets/gocfd/DG2D"
	"github.com/notargets/gocfd/utils"
)

type SolutionLimiter struct {
	Element              *DG2D.LagrangeElement2D
	Tris                 *DG2D.Triangulation
	Partitions           *PartitionMap
	ShockFinder          []*ModeAliasShockFinder // Sharded
	UElement, dUdr, dUds []utils.Matrix          // Sharded scratch areas for assembly and testing of solution values
}

func NewSolutionLimiter(dfr *DG2D.DFR2D, pm *PartitionMap) (bjl *SolutionLimiter) {
	var (
		Np       = dfr.SolutionElement.Np
		Nthreads = pm.ParallelDegree
	)
	bjl = &SolutionLimiter{
		Element:     dfr.SolutionElement,
		Tris:        dfr.Tris,
		ShockFinder: make([]*ModeAliasShockFinder, Nthreads),
		Partitions:  pm,
		// Sharded working matrices
		UElement: make([]utils.Matrix, Nthreads),
		dUdr:     make([]utils.Matrix, Nthreads),
		dUds:     make([]utils.Matrix, Nthreads),
	}
	for np := 0; np < Nthreads; np++ {
		bjl.ShockFinder[np] = NewAliasShockFinder(dfr.SolutionElement)
		bjl.UElement[np] = utils.NewMatrix(Np, 1)
		bjl.dUdr[np] = utils.NewMatrix(Np, 1)
		bjl.dUds[np] = utils.NewMatrix(Np, 1)
	}
	return
}

func (bjl *SolutionLimiter) LimitSolution(myThread int, Qall, Residual [][4]utils.Matrix) {
	var (
		Q        = Qall[myThread]
		Np, Kmax = Q[0].Dims()
		UE       = bjl.UElement[myThread]
	)
	for k := 0; k < Kmax; k++ {
		for i := 0; i < Np; i++ {
			ind := k + Kmax*i
			UE.DataP[i] = Q[3].DataP[ind] // Use Energy as the indicator basis
		}
		if bjl.ShockFinder[myThread].ElementHasShock(UE.DataP) { // Element has a shock
			/*
				kkk, _, _ := bjl.Partitions.GetLocalK(k)
				fmt.Printf("limiting element %d\n", kkk)
			*/
			bjl.limitScalarField(k, myThread, Qall)
			for n := 0; n < 4; n++ {
				for i := 0; i < Np; i++ {
					ind := k + Kmax*i
					Residual[myThread][n].DataP[ind] = 0.
				}
			}
		}
	}
}

func (bjl *SolutionLimiter) limitScalarField(k, myThread int, Qall [][4]utils.Matrix) {
	var (
		Np, Kmax   = Qall[myThread][0].Dims()
		Dr, Ds     = bjl.Element.Dr, bjl.Element.Ds
		UE         = bjl.UElement[myThread]
		dUdr, dUds = bjl.dUdr[myThread], bjl.dUds[myThread]
		min, max   = math.Min, math.Max
	)
	getElAvg := func(f utils.Matrix, kkk, kMx int) (ave float64) {
		for i := 0; i < Np; i++ {
			ind := kkk + kMx*i
			ave += f.DataP[ind]
		}
		ave /= float64(Np)
		return
	}
	psiCalc := func(corner int, Umin, Umax, Uave, dUdrAve, dUdsAve float64) (psi float64) {
		var (
			del2 float64
		)
		/*
			For each corner of the unit triangle, the vector from center to corner is:
				ri[0] = [ -2/3, -2/3 ]
				ri[1] = [ 4/3, -2/3 ]
				ri[2] = [ -2/3, 4/3 ]
		*/
		switch corner {
		case 0:
			del2 = -2. / 3. * (dUdrAve + dUdsAve)
		case 1:
			del2 = (4./3.)*dUdrAve - (2./3.)*dUdsAve
		case 2:
			del2 = -(2./3.)*dUdrAve + (4./3.)*dUdsAve
		}
		oodel2 := 1. / del2
		// Calculate limiter function Psi
		switch {
		case del2 > 0:
			psi = min(1, oodel2*(Umax-Uave))
		case del2 == 0:
			psi = 1
		case del2 < 0:
			psi = min(1, oodel2*(Umin-Uave))
		}
		return
	}
	for n := 0; n < 4; n++ {
		var (
			U                = Qall[myThread][n]
			Uave, Umin, Umax float64
		)
		// Apply limiting procedure
		// Get average and min/max solution value for element and neighbors
		Uave = getElAvg(U, k, Kmax)
		Umin, Umax = Uave, Uave
		// Loop over connected tris to get Umin, Umax
		for ii := 0; ii < 3; ii++ {
			kk := bjl.Tris.EtoE[k][ii]
			if kk != -1 {
				remoteK, remoteKmax, rThread := bjl.Partitions.GetLocalK(kk)
				UU := getElAvg(Qall[rThread][n], remoteK, remoteKmax)
				Umax = max(UU, Umax)
				Umin = min(UU, Umin)
			}
		}
		for i := 0; i < Np; i++ {
			ind := k + Kmax*i
			UE.DataP[i] = U.DataP[ind]
		}
		// Obtain average gradient of this cell
		dUdrAve, dUdsAve := Dr.Mul(UE, dUdr).Avg(), Ds.Mul(UE, dUds).Avg()
		// Form psi as the minimum of all three corners
		var psi float64
		psi = 10000.
		for nn := 0; nn < 3; nn++ {
			psi = min(psi, psiCalc(nn, Umin, Umax, Uave, dUdrAve, dUdsAve))
		}
		// Limit the solution using psi and the average gradient
		for i := 0; i < Np; i++ {
			// Vector from node points to center of element
			dR, dS := bjl.Element.R.DataP[i]-(-1./3), bjl.Element.S.DataP[i]-(-1./3.)
			ind := k + Kmax*i
			U.DataP[ind] = Uave + psi*(dR*dUdrAve+dS*dUdsAve)
		}
	}
}

type ModeAliasShockFinder struct {
	Element *DG2D.LagrangeElement2D
	Clipper utils.Matrix // Matrix used to clip the topmost mode from the solution polynomial, used in shockfinder
	Np      int
	q, qalt utils.Matrix // scratch storage for evaluating the moment
}

func NewAliasShockFinder(element *DG2D.LagrangeElement2D) (sf *ModeAliasShockFinder) {
	var (
		Np = element.Np
	)
	sf = &ModeAliasShockFinder{
		Element: element,
		Np:      Np,
		q:       utils.NewMatrix(Np, 1),
		qalt:    utils.NewMatrix(Np, 1),
	}
	data := make([]float64, Np)
	for i := 0; i < Np; i++ {
		if i != Np-1 {
			data[i] = 1.
		} else {
			data[i] = 0.
		}
	}
	diag := utils.NewDiagMatrix(Np, data)
	/*
		The "Clipper" matrix drops the last mode from the polynomial and forms an alternative field of values at the node
		points based on a polynomial with one less term. In other words, if we have a polynomial of degree "p", expressed
		as values at Np node points, multiplying the Node point values vector by Clipper produces an alternative version
		of the node values based on truncating the last polynomial mode.
	*/
	sf.Clipper = element.V.Mul(diag).Mul(element.Vinv)
	return
}

func (sf *ModeAliasShockFinder) ElementHasShock(q []float64) (i bool) {
	// Zhiqiang uses a threshold of sigma<0.99 to indicate "troubled cell"
	if sf.ShockIndicator(q) < 0.99 {
		i = true
	}
	return
}

func (sf *ModeAliasShockFinder) ShockIndicator(q []float64) (sigma float64) {
	/*
		Original method by Persson, constants chosen to match Zhiqiang, et. al.
	*/
	var (
		Se          = math.Log10(sf.moment(q))
		k           = float64(sf.Element.N)
		kappa       = 4.
		C0          = 3.
		S0          = -C0 * math.Log(k)
		left, right = S0 - kappa, S0 + kappa
		ookappa     = 1. / kappa
	)
	switch {
	case Se < left:
		sigma = 1.
	case Se >= left && Se < right:
		sigma = 0.5 * (1. - math.Sin(0.5*math.Pi*ookappa*(Se-S0)))
	case Se >= right:
		sigma = 0.
	}
	return
}

func (sf *ModeAliasShockFinder) moment(q []float64) (m float64) {
	var (
		qd, qaltd = sf.q.DataP, sf.qalt.DataP
	)
	if len(q) != sf.Np {
		err := fmt.Errorf("incorrect dimension of solution vector, should be %d is %d",
			sf.Np, len(q))
		panic(err)
	}
	/*
		Evaluate the L2 moment of (q - qalt) over the element, where qalt is the truncated version of q
		Here we don't bother using quadrature, we do a simple sum
	*/
	copy(sf.q.DataP, q)
	sf.qalt = sf.Clipper.Mul(sf.q, sf.qalt)
	for i := 0; i < sf.Np; i++ {
		t1 := qd[i] - qaltd[i]
		m += t1 * t1 / (qd[i] * qd[i])
	}
	return
}
