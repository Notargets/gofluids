package Euler2D

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/notargets/gocfd/types"

	"github.com/notargets/gocfd/DG2D"

	"github.com/notargets/gocfd/utils"
)

/*
	In the DFR scheme, we have two sets of points:
		- Solution points (inside element)
		- Flux Points (on element faces)
	The flux element operates on both sets of points, the solution element only operates on the internal solution points
*/
type Euler struct {
	// Input parameters
	MeshFile          string
	CFL, FinalTime    float64
	FS                *FreeStream
	dfr               *DG2D.DFR2D
	Q                 [4]utils.Matrix // Solution variables, stored at solution point locations, Np_solution x K
	DT                utils.Matrix    // Local time step for steady state solution
	Q_Face            [4]utils.Matrix // Solution variables, interpolated to and stored at edge point locations, Np_edge x K
	F_RT_DOF          [4]utils.Matrix // Normal Projected Flux, stored at flux/solution point locations, Np_flux x K
	chart             ChartState
	FluxCalcAlgo      FluxType
	Case              InitType
	AnalyticSolution  ExactState
	FluxCalcMock      func(Q [4]float64) (Fx, Fy [4]float64) // For testing
	SortedEdgeKeys    EdgeKeySlice
	ParallelDegree    int // Number of go routines to use for parallel execution
	LocalTimeStepping bool
	MaxIterations     int
}

func NewEuler(FinalTime float64, N int, meshFile string, CFL float64,
	fluxType FluxType, Case InitType, ProcLimit int,
	Minf, Gamma, Alpha float64, LocalTime bool, MaxIterations int, plotMesh, verbose bool) (c *Euler) {
	c = &Euler{
		MeshFile:          meshFile,
		CFL:               CFL,
		FinalTime:         FinalTime,
		FluxCalcAlgo:      fluxType,
		Case:              Case,
		LocalTimeStepping: LocalTime,
		MaxIterations:     MaxIterations,
		FS:                NewFreeStream(Minf, Gamma, Alpha),
	}
	c.FluxCalcMock = c.FluxCalc
	if ProcLimit != 0 {
		c.ParallelDegree = ProcLimit
	} else {
		c.ParallelDegree = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(runtime.NumCPU())

	if len(meshFile) == 0 {
		return
	}

	c.dfr = DG2D.NewDFR2D(N, plotMesh, meshFile)
	c.InitializeMemory()
	if c.ParallelDegree > c.dfr.K {
		c.ParallelDegree = 1
	}

	if verbose {
		fmt.Printf("Euler Equations in 2 Dimensions\n")
		fmt.Printf("Using %d go routines in parallel\n", runtime.NumCPU())
		fmt.Printf("Solving %s\n", c.Case.Print())
		if c.Case == FREESTREAM {
			fmt.Printf("Mach Infinity = %8.5f, Angle of Attack = %8.5f\n", Minf, Alpha)
		}
	}

	// Setup the key for edge calculations, useful for parallelizing the process
	c.SortedEdgeKeys = make(EdgeKeySlice, len(c.dfr.Tris.Edges))
	var i int
	for en := range c.dfr.Tris.Edges {
		c.SortedEdgeKeys[i] = en
		i++
	}
	c.SortedEdgeKeys.Sort()
	switch c.Case {
	case FREESTREAM:
		c.InitializeFS()
	case IVORTEX:
		c.FS.Qinf = [4]float64{1, 1, 0, 3}
		c.FS.Pinf = c.FS.GetFlowFunction(c.FS.Qinf, StaticPressure)
		c.FS.QQinf = c.FS.GetFlowFunction(c.FS.Qinf, DynamicPressure)
		c.FS.Cinf = c.FS.GetFlowFunction(c.FS.Qinf, SoundSpeed)
		c.AnalyticSolution, c.Q = c.InitializeIVortex(c.dfr.SolutionX, c.dfr.SolutionY)
		// Set "Wall" BCs to IVortex
		var count int
		for _, e := range c.dfr.Tris.Edges {
			if e.BCType == types.BC_Wall {
				count++
				e.BCType = types.BC_IVortex
			}
		}
		if verbose {
			fmt.Printf("\tReplaced %d Wall boundary conditions with analytic BC_IVortex\n", count)
		}
	default:
		panic("unknown case type")
	}
	if verbose {
		fmt.Printf("Algorithm: %s\n", c.FluxCalcAlgo.Print())
		fmt.Printf("CFL = %8.4f, Polynomial Degree N = %d (1 is linear), Num Elements K = %d\n\n\n", CFL, N, c.dfr.K)
	}
	return
}

func (c *Euler) Solve(pm *PlotMeta) {
	var (
		FinalTime            = c.FinalTime
		Time, dt             float64
		Q1, Q2, Q3, Residual [4]utils.Matrix
		steps                int
		finished             bool
		Np                   = c.dfr.SolutionElement.Np
		Kmax                 = c.dfr.K
		plotQ                = pm.Plot
	)
	c.PrintInitialization(FinalTime)
	// Initialize memory for RHS
	for n := 0; n < 4; n++ {
		Q1[n] = utils.NewMatrix(Np, Kmax)
		Q2[n] = utils.NewMatrix(Np, Kmax)
		Q3[n] = utils.NewMatrix(Np, Kmax)
	}
	start := time.Now()
	for !finished {
		Residual, dt = c.RungeKutta4SSP(Time, Q1, Q2, Q3)
		steps++
		Time += dt
		finished = c.CheckIfFinished(Time, FinalTime, steps)
		if finished || steps%pm.StepsBeforePlot == 0 || steps == 1 {
			c.PrintUpdate(Time, dt, steps, Residual, plotQ, pm)
		}
	}
	elapsed := time.Now().Sub(start)
	c.PrintFinal(elapsed, steps)
}

func (c *Euler) InitializeMemory() {
	var (
		K          = c.dfr.K
		Nedge      = c.dfr.FluxElement.Nedge
		NpFlux     = c.dfr.FluxElement.Np
		NpSolution = c.dfr.SolutionElement.Np
	)
	for n := 0; n < 4; n++ {
		c.Q_Face[n] = utils.NewMatrix(Nedge*3, K)
		c.F_RT_DOF[n] = utils.NewMatrix(NpFlux, K)
		c.DT = utils.NewMatrix(NpSolution, K) // Local time step
	}
}

func (c *Euler) RungeKutta3SSP(Time float64, Q1, Q2 [4]utils.Matrix) (Residual [4]utils.Matrix, dt float64) {
	var (
		Np   = c.dfr.SolutionElement.Np
		Kmax = c.dfr.K
		wg   = sync.WaitGroup{}
	)
	for n := 0; n < 4; n++ {
		Residual[n] = Q1[n] // optimize memory using an alias
	}
	// Get pointers to the underlying data for each matrix
	qD := Get4DP(c.Q)
	q1D := Get4DP(Q1)
	q2D := Get4DP(Q2)
	resD := Get4DP(Residual)
	dtD := c.DT.Data()

	rhsQ := c.RHS(c.Q, Time)
	rhsD := Get4DP(rhsQ)
	dt = c.CalculateDT()
	if Time+dt > c.FinalTime {
		dt = c.FinalTime - Time
	}
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := utils.Split1D(Kmax*Np, c.ParallelDegree, np)
		wg.Add(1)
		go func(ind, end int) {
			var dT = dt
			for n := 0; n < 4; n++ {
				for i := ind; i < end; i++ {
					if c.LocalTimeStepping {
						dT = dtD[i]
					}
					q1D[n][i] = qD[n][i] + rhsD[n][i]*dT
				}
			}
			wg.Done()
		}(ind, end)
	}
	wg.Wait()
	rhsQ = c.RHS(Q1, Time)
	rhsD = Get4DP(rhsQ)
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := utils.Split1D(Kmax*Np, c.ParallelDegree, np)
		wg.Add(1)
		go func(ind, end int) {
			var dT = dt
			for n := 0; n < 4; n++ {
				for i := ind; i < end; i++ {
					if c.LocalTimeStepping {
						dT = dtD[i]
					}
					q2D[n][i] = 0.25 * (q1D[n][i] + 3*qD[n][i] + rhsD[n][i]*dT)
				}
			}
			wg.Done()
		}(ind, end)
	}
	wg.Wait()
	rhsQ = c.RHS(Q2, Time)
	rhsD = Get4DP(rhsQ)
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := utils.Split1D(Kmax*Np, c.ParallelDegree, np)
		wg.Add(1)
		go func(ind, end int) {
			var dT = dt
			for n := 0; n < 4; n++ {
				for i := ind; i < end; i++ {
					if c.LocalTimeStepping {
						dT = dtD[i]
					}
					resD[n][i] = (2. / 3.) * (q2D[n][i] - qD[n][i] + 0.5*rhsD[n][i]*dT)
					qD[n][i] += resD[n][i]
				}
			}
			wg.Done()
		}(ind, end)
	}
	wg.Wait()
	return
}

func (c *Euler) RungeKutta4SSP(Time float64, Q1, Q2, Q3 [4]utils.Matrix) (Residual [4]utils.Matrix, dt float64) {
	var (
		Np   = c.dfr.SolutionElement.Np
		Kmax = c.dfr.K
		wg   = sync.WaitGroup{}
	)
	for n := 0; n < 4; n++ {
		Residual[n] = Q1[n] // optimize memory using an alias
	}
	// Get pointers to the underlying data for each matrix
	qD := Get4DP(c.Q)
	q1D := Get4DP(Q1)
	q2D := Get4DP(Q2)
	q3D := Get4DP(Q3)
	resD := Get4DP(Residual)
	dtD := c.DT.Data()

	rhsQ := c.RHS(c.Q, Time)
	rhsD := Get4DP(rhsQ)
	dt = c.CalculateDT()
	if Time+dt > c.FinalTime {
		dt = c.FinalTime - Time
	}
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := utils.Split1D(Kmax*Np, c.ParallelDegree, np)
		wg.Add(1)
		go func(ind, end int) {
			var dT = dt
			for n := 0; n < 4; n++ {
				for i := ind; i < end; i++ {
					if c.LocalTimeStepping {
						dT = dtD[i]
					}
					q1D[n][i] = qD[n][i] + 0.5*rhsD[n][i]*dT
				}
			}
			wg.Done()
		}(ind, end)
	}
	wg.Wait()
	rhsQ = c.RHS(Q1, Time)
	rhsD = Get4DP(rhsQ)
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := utils.Split1D(Kmax*Np, c.ParallelDegree, np)
		wg.Add(1)
		go func(ind, end int) {
			var dT = dt
			for n := 0; n < 4; n++ {
				for i := ind; i < end; i++ {
					if c.LocalTimeStepping {
						dT = dtD[i]
					}
					q2D[n][i] = q1D[n][i] + 0.25*rhsD[n][i]*dT
				}
			}
			wg.Done()
		}(ind, end)
	}
	wg.Wait()
	rhsQ = c.RHS(Q2, Time)
	rhsD = Get4DP(rhsQ)
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := utils.Split1D(Kmax*Np, c.ParallelDegree, np)
		wg.Add(1)
		go func(ind, end int) {
			var dT = dt
			for n := 0; n < 4; n++ {
				for i := ind; i < end; i++ {
					if c.LocalTimeStepping {
						dT = dtD[i]
					}
					q3D[n][i] = (1. / 3.) * (2*qD[n][i] + q2D[n][i] + rhsD[n][i]*dT)
				}
			}
			wg.Done()
		}(ind, end)
	}
	wg.Wait()
	rhsQ = c.RHS(Q3, Time)
	rhsD = Get4DP(rhsQ)
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := utils.Split1D(Kmax*Np, c.ParallelDegree, np)
		wg.Add(1)
		go func(ind, end int) {
			var dT = dt
			for n := 0; n < 4; n++ {
				for i := ind; i < end; i++ {
					if c.LocalTimeStepping {
						dT = dtD[i]
					}
					resD[n][i] = q3D[n][i] + 0.25*rhsD[n][i]*dT - qD[n][i]
					qD[n][i] += resD[n][i]
				}
			}
			wg.Done()
		}(ind, end)
	}
	wg.Wait()
	return
}

func (c *Euler) CalculateDT() (dt float64) {
	var (
		Np1      = c.dfr.N + 1
		Np12     = float64(Np1 * Np1)
		wsMaxAll = -math.MaxFloat64
		wsMax    = make([]float64, c.ParallelDegree)
		wg       = sync.WaitGroup{}
		qfD      = Get4DP(c.Q_Face)
		JdetD    = c.dfr.Jdet.Data()
	)
	// Setup max wavespeed before loop
	dtD := c.DT.Data()
	for k := 0; k < c.dfr.K; k++ {
		dtD[k] = -100
	}
	for nn := 0; nn < c.ParallelDegree; nn++ {
		wsMax[nn] = wsMaxAll
	}
	// Loop over all edges, calculating max wavespeed
	for nn := 0; nn < c.ParallelDegree; nn++ {
		ind, end := c.split1D(len(c.SortedEdgeKeys), nn)
		wg.Add(1)
		go func(ind, end, nn int) {
			for ii := ind; ii < end; ii++ {
				edgeKey := c.SortedEdgeKeys[ii]
				e := c.dfr.Tris.Edges[edgeKey]
				var (
					edgeLen = e.GetEdgeLength()
					Nedge   = c.dfr.FluxElement.Nedge
				)
				conn := 0
				var (
					k       = int(e.ConnectedTris[conn])
					edgeNum = int(e.ConnectedTriEdgeNumber[conn])
					shift   = edgeNum * Nedge
				)
				Jdet := JdetD[k]
				// fmt.Printf("N, Np12, edgelen, Jdet = %d,%8.5f,%8.5f,%8.5f\n", c.dfr.N, Np12, edgeLen, Jdet)
				fs := 0.5 * Np12 * edgeLen / Jdet
				edgeMax := -100.
				for i := shift; i < shift+Nedge; i++ {
					qq := c.GetQQ(k, i, qfD)
					C := c.FS.GetFlowFunction(qq, SoundSpeed)
					U := c.FS.GetFlowFunction(qq, Velocity)
					waveSpeed := fs * (U + C)
					wsMax[nn] = math.Max(waveSpeed, wsMax[nn])
					if waveSpeed > edgeMax {
						edgeMax = waveSpeed
					}
				}
				if edgeMax > dtD[k] {
					dtD[k] = edgeMax
				}
				if e.NumConnectedTris == 2 { // Add the wavespeed to the other tri connected to this edge if needed
					k = int(e.ConnectedTris[1])
					if edgeMax > dtD[k] {
						dtD[k] = edgeMax
					}
				}
			}
			wg.Done()
		}(ind, end, nn)
	}
	wg.Wait()
	// Replicate local time step to the other solution points for each k
	for k := 0; k < c.dfr.K; k++ {
		dtD[k] = c.CFL / dtD[k]
	}
	for i := 1; i < c.dfr.SolutionElement.Np; i++ {
		for k := 0; k < c.dfr.K; k++ {
			ind := k + c.dfr.K*i
			dtD[ind] = dtD[k]
		}
	}
	for nn := 0; nn < c.ParallelDegree; nn++ {
		wsMaxAll = math.Max(wsMaxAll, wsMax[nn])
	}
	dt = c.CFL / wsMaxAll
	return
}

func (c *Euler) RHS(Q [4]utils.Matrix, Time float64) (RHSCalc [4]utils.Matrix) {
	/*
				Calculate the RHS of the equation:
				dQ/dt = -div(F,G)
				Where:
					Q = [rho, rhoU, rhoV, E]
					F = [rhoU, rhoU*u+p, rhoV*u, u*(E+p)]
					G = [rhoV, rhoU*v, rhoV*v+p, v*(E+p)]

		    	The divergence div(F,G) is calculated using a Raviart Thomas finite element with flux (F,G) values on the faces
				of the element "injected" via calculation of a physical flux on those faces, and the (F,G) values in the interior
				of the element taken directly from the solution values (Q).
	*/
	c.AssembleRTNormalFlux(Q, Time) // Assembles F_RT_DOF for use in calculations using RT element
	var wg = sync.WaitGroup{}
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func(n int) {
			RHSCalc[n] = c.dfr.FluxElement.DivInt.Mul(c.F_RT_DOF[n]) // Calculate divergence for the internal node points
			c.DivideByJacobian(c.dfr.FluxElement.Nint, RHSCalc[n].Data(), -1)
			wg.Done()
		}(n)
	}
	wg.Wait()
	return
}

func (c *Euler) DivideByJacobian(Nmax int, data []float64, scale float64) {
	var (
		Kmax  = c.dfr.K
		JdetD = c.dfr.Jdet.Data()
	)
	for k := 0; k < Kmax; k++ {
		Jdet := JdetD[k]
		for i := 0; i < Nmax; i++ {
			ind := k + i*Kmax
			data[ind] /= (Jdet * scale) // Multiply divergence by -1 to produce the RHS
		}
	}
}

func (c *Euler) AssembleRTNormalFlux(Q [4]utils.Matrix, Time float64) {
	var (
		Kmax  = c.dfr.K
		Nedge = c.dfr.FluxElement.Nedge
		Np    = c.dfr.FluxElement.Np
		qfD   = Get4DP(c.Q_Face)
		fdofD = Get4DP(c.F_RT_DOF)
	)
	/*
		Solver approach:
		0) Solution is stored on sol points as Q
		0a) Flux is computed and stored in X, Y component projections in the 2*Nint front of F_RT_DOF
		1) Solution is extrapolated to edge points in Q_Face from Q
		2) Edges are traversed, flux is calculated and projected onto edge face normals, scaled and placed into F_RT_DOF
	*/
	/*
		Zero out DOF storage and Q_Face to promote easier bug avoidance
	*/
	for n := 0; n < 4; n++ {
		for i := 0; i < Kmax*Nedge; i++ {
			qfD[n][i] = 0.
		}
		for i := 0; i < Kmax*Np; i++ {
			fdofD[n][i] = 0.
		}
	}
	c.SetNormalFluxInternal(Q)           // Updates F_RT_DOF with values from Q
	c.InterpolateSolutionToEdges(Q)      // Interpolates Q_Face values from Q
	c.ParallelSetNormalFluxOnEdges(Time) // Updates F_RT_DOG with values from edges, including BCs and connected tris
}

func (c *Euler) SetNormalFluxInternal(Q [4]utils.Matrix) {
	var (
		Kmax  = c.dfr.K
		Nint  = c.dfr.FluxElement.Nint
		wg    = sync.WaitGroup{}
		qD    = Get4DP(Q)
		fdofD = Get4DP(c.F_RT_DOF)
	)
	// Calculate flux and project into R and S (transformed) directions for the internal points
	//for k := 0; k < Kmax; k++ {
	for np := 0; np < c.ParallelDegree; np++ {
		ind, end := c.split1D(Kmax, np)
		wg.Add(1)
		go func() {
			for k := ind; k < end; k++ {
				for i := 0; i < Nint; i++ {
					ind := k + i*Kmax
					ind2 := k + (i+Nint)*Kmax
					Fr, Fs := c.CalculateFluxTransformed(k, i, qD)
					for n := 0; n < 4; n++ {
						fdofD[n][ind], fdofD[n][ind2] = Fr[n], Fs[n]
					}
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func (c *Euler) CheckIfFinished(Time, FinalTime float64, steps int) (finished bool) {
	if Time >= FinalTime || steps >= c.MaxIterations {
		finished = true
	}
	return
}
func (c *Euler) PrintInitialization(FinalTime float64) {
	fmt.Printf("Using mesh from file: [%s]\n", c.MeshFile)
	if c.LocalTimeStepping {
		fmt.Printf("Solving until Max Iterations = %d\n", c.MaxIterations)
		fmt.Printf("    iter                ")
	} else {
		fmt.Printf("Solving until finaltime = %8.5f\n", FinalTime)
		fmt.Printf("    iter    time  min_dt")
	}
	fmt.Printf("       Res0       Res1       Res2")
	fmt.Printf("       Res3         L1         L2\n")
}
func (c *Euler) PrintUpdate(Time, dt float64, steps int, Residual [4]utils.Matrix, plotQ bool, pm *PlotMeta) {
	format := "%11.4e"
	if plotQ {
		c.PlotQ(c.Q, pm) // wait till we implement time iterative frame updates
	}
	if c.LocalTimeStepping {
		fmt.Printf("%10d              ", steps)
	} else {
		fmt.Printf("%8d%8.5f%8.5f", steps, Time, dt)
	}
	var l1, l2 float64
	for n := 0; n < 4; n++ {
		maxR := Residual[n].Max()
		fmt.Printf(format, maxR)
		if maxR > l1 {
			l1 = maxR
		}
		l2 += maxR * maxR
	}
	fmt.Printf(format, l1)
	fmt.Printf(format, math.Sqrt(l2)/4.)
	fmt.Printf("\n")
}
func (c *Euler) PrintFinal(elapsed time.Duration, steps int) {
	rate := float64(elapsed.Microseconds()) / (float64(c.dfr.K * steps))
	fmt.Printf("\nRate of execution = %8.5f us/(element*iteration) over %d iterations\n", rate, steps)
}
