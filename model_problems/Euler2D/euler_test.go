package Euler2D

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/notargets/gocfd/utils"
)

func TestEuler(t *testing.T) {
	{ // Test interpolation of solution to edges for all supported orders
		Nmax := 7
		for N := 1; N <= Nmax; N++ {
			c := NewEuler(1, N, "../../DG2D/test_tris_5.neu", 1, FLUX_Average, FREESTREAM, false, false)
			Kmax := c.dfr.K
			Nint := c.dfr.FluxElement.Nint
			Nedge := c.dfr.FluxElement.Nedge
			for n := 0; n < 4; n++ {
				for i := 0; i < Nint; i++ {
					for k := 0; k < Kmax; k++ {
						ind := k + i*Kmax
						c.Q[n].Data()[ind] = float64(k + 1)
					}
				}
			}
			// Interpolate from solution points to edges using precomputed interpolation matrix
			for n := 0; n < 4; n++ {
				c.Q_Face[n] = c.dfr.FluxEdgeInterpMatrix.Mul(c.Q[n])
			}
			for n := 0; n < 4; n++ {
				for i := 0; i < 3*Nedge; i++ {
					for k := 0; k < Kmax; k++ {
						ind := k + i*Kmax
						assert.True(t, near(float64(k+1), c.Q_Face[n].Data()[ind], 0.000001))
					}
				}
			}
		}
	}
	{ // Test solution process
		/*
			Solver approach:
			0) Solution is stored on sol points as Q
			0a) Flux is computed and stored in X, Y component projections in the 2*Nint front of F_RT_DOF
			1) Solution is extrapolated to edge points in Q_Face from Q
			2) Edges are traversed, flux is calculated and projected onto edge face normals, scaled and placed into F_RT_DOF
		*/
		Nmax := 7
		for N := 1; N <= Nmax; N++ {
			c := NewEuler(1, N, "../../DG2D/test_tris_5.neu", 1, FLUX_Average, FREESTREAM, false, false)
			Kmax := c.dfr.K
			Nint := c.dfr.FluxElement.Nint
			Nedge := c.dfr.FluxElement.Nedge
			NpFlux := c.dfr.FluxElement.Np // Np = 2*Nint+3*Nedge
			// Mark the initial state with the element number
			for i := 0; i < Nint; i++ {
				for k := 0; k < Kmax; k++ {
					ind := k + i*Kmax
					c.Q[0].Data()[ind] = float64(k + 1)
					c.Q[1].Data()[ind] = 0.1 * float64(k+1)
					c.Q[2].Data()[ind] = 0.05 * float64(k+1)
					c.Q[3].Data()[ind] = 2.00 * float64(k+1)
				}
			}
			// Flux values for later checks are invariant with i (i=0)
			Fr_check, Fs_check := make([][4]float64, Kmax), make([][4]float64, Kmax)
			for k := 0; k < Kmax; k++ {
				Fr_check[k], Fs_check[k] = c.CalculateFluxTransformed(k, 0, c.Q)
			}
			// Interpolate from solution points to edges using precomputed interpolation matrix
			for n := 0; n < 4; n++ {
				c.Q_Face[n] = c.dfr.FluxEdgeInterpMatrix.Mul(c.Q[n])
			}
			// Calculate flux and project into R and S (transformed) directions
			for n := 0; n < 4; n++ {
				for i := 0; i < Nint; i++ {
					for k := 0; k < c.dfr.K; k++ {
						ind := k + i*Kmax
						Fr, Fs := c.CalculateFluxTransformed(k, i, c.Q)
						rtD := c.F_RT_DOF[n].Data()
						rtD[ind], rtD[ind+Nint*Kmax] = Fr[n], Fs[n]
					}
				}
				// Check to see that the expected values are in the right place (the internal locations)
				for k := 0; k < Kmax; k++ {
					val0, val1 := Fr_check[k][n], Fs_check[k][n]
					is := k * NpFlux
					assert.True(t, nearVecScalar(c.F_RT_DOF[n].Transpose().Data()[is:is+Nint],
						val0, 0.000001))
					is += Nint
					assert.True(t, nearVecScalar(c.F_RT_DOF[n].Transpose().Data()[is:is+Nint],
						val1, 0.000001))
				}
				// Set normal flux to a simple addition of the two sides to use as a check in assert()
				for k := 0; k < Kmax; k++ {
					for i := 0; i < 3*Nedge; i++ {
						ind := k + (2*Nint+i)*Kmax
						Fr, Fs := c.CalculateFluxTransformed(k, i, c.Q_Face)
						rtD := c.F_RT_DOF[n].Data()
						rtD[ind] = Fr[n] + Fs[n]
					}
				}
				// Check to see that the expected values are in the right place (the edge locations)
				for k := 0; k < Kmax; k++ {
					val := Fr_check[k][n] + Fs_check[k][n]
					is := k * NpFlux
					ie := (k + 1) * NpFlux
					assert.True(t, nearVecScalar(c.F_RT_DOF[n].Transpose().Data()[is+2*Nint:ie],
						val, 0.000001))
				}
			}
		}
	}
	{ // Test solution process part 2 - Freestream divergence should be zero
		Nmax := 7
		for N := 1; N <= Nmax; N++ {
			c := NewEuler(1, N, "../../DG2D/test_tris_5.neu", 1, FLUX_Average, FREESTREAM, false, false)
			c.SetNormalFluxInternal(c.Q)
			c.InterpolateSolutionToEdges(c.Q)
			c.SetNormalFluxOnEdges(0)
			Kmax := c.dfr.K
			Nint := c.dfr.FluxElement.Nint
			// Check that freestream divergence on this mesh is zero
			for n := 0; n < 4; n++ {
				var div utils.Matrix
				div = c.dfr.FluxElement.DivInt.Mul(c.F_RT_DOF[n])
				for k := 0; k < Kmax; k++ {
					_, _, Jdet := c.dfr.GetJacobian(k)
					for i := 0; i < Nint; i++ {
						ind := k + i*Kmax
						div.Data()[ind] /= Jdet
					}
				}
				assert.True(t, nearVecScalar(div.Data(), 0., 0.000001))
			}
		}
	}
	{ // Test divergence of polynomial initial condition against analytic values
		/*
			Note: the Polynomial flux is asymmetric around the X and Y axes - it uses abs(x) and abs(y)
			Elements should not straddle the axes if a perfect polynomial flux capture is needed
		*/
		Nmax := 7
		for N := 1; N <= Nmax; N++ {
			plotMesh := false
			// Single triangle test case
			var c *Euler
			c = NewEuler(1, N, "../../DG2D/test_tris_1tri.neu", 1, FLUX_Average, FREESTREAM, plotMesh, false)
			CheckFlux0(c, t)
			// Two widely separated triangles - no shared faces
			c = NewEuler(1, N, "../../DG2D/test_tris_two.neu", 1, FLUX_Average, FREESTREAM, plotMesh, false)
			CheckFlux0(c, t)
			// Two widely separated triangles - no shared faces - one tri listed in reverse order
			c = NewEuler(1, N, "../../DG2D/test_tris_twoR.neu", 1, FLUX_Average, FREESTREAM, plotMesh, false)
			CheckFlux0(c, t)
			// Connected tris, sharing one edge
			//plotMesh = true
			c = NewEuler(1, N, "../../DG2D/test_tris_6.neu", 1, FLUX_Average, FREESTREAM, plotMesh, false)
			CheckFlux0(c, t)
		}
	}
	{ // Test divergence of Isentropic Vortex initial condition against analytic values - density equation only
		N := 1
		plotMesh := false
		// c := NewEuler(1, N, "../../DG2D/vortexA04.neu", 1, FLUX_Average, IVORTEX, plotMesh, false)
		c := NewEuler(1, N, "../../DG2D/test_tris_6.neu", 1, FLUX_Average, IVORTEX, plotMesh, false)
		X, Y := c.dfr.FluxX, c.dfr.FluxY
		Kmax := c.dfr.K
		Nint := c.dfr.FluxElement.Nint
		c.SetNormalFluxInternal(c.Q)
		c.InterpolateSolutionToEdges(c.Q)
		c.SetNormalFluxOnEdges(0)
		var div utils.Matrix
		// Density is the easiest equation to match with a polynomial
		n := 0
		fmt.Printf("component[%d]\n", n)
		div = c.dfr.FluxElement.DivInt.Mul(c.F_RT_DOF[n])
		for k := 0; k < Kmax; k++ {
			_, _, Jdet := c.dfr.GetJacobian(k)
			for i := 0; i < Nint; i++ {
				ind := k + i*Kmax
				div.Data()[ind] /= Jdet
			}
		}
		// Get the analytic values of divergence for comparison
		for k := 0; k < Kmax; k++ {
			for i := 0; i < Nint; i++ {
				ind := k + i*Kmax
				x, y := X.Data()[ind], Y.Data()[ind]
				qc1, qc2, qc3, qc4 := c.AnalyticSolution.GetStateC(0, x, y)
				q1, q2, q3, q4 := c.Q[0].Data()[ind], c.Q[1].Data()[ind], c.Q[2].Data()[ind], c.Q[3].Data()[ind]
				assert.True(t, nearVec([]float64{q1, q2, q3, q4}, []float64{qc1, qc2, qc3, qc4}, 0.000001))
				divC := c.AnalyticSolution.GetDivergence(0, x, y)
				divCalc := div.Data()[ind]
				// fmt.Printf("div[%d][%d,%d] = %8.5f\n", n, k, i, divCalc)
				assert.True(t, near(divCalc/qc1, divC[n]/qc1, 0.001)) // 0.1 percent match
			}
		}
	}
	{ // Test solver
		N := 2
		plotMesh := false
		plotQ := false
		c := NewEuler(1.0, N, "../../DG2D/vortexA04.neu", 0.2, FLUX_Average, IVORTEX, plotMesh, true)
		c.Solve(plotQ)
	}
}

func PrintQ(Q [4]utils.Matrix, l string) {
	var (
		label string
	)
	for ii := 0; ii < 4; ii++ {
		switch ii {
		case 0:
			label = l + "_0"
		case 1:
			label = l + "_1"
		case 2:
			label = l + "_2"
		case 3:
			label = l + "_3"
		}
		fmt.Println(Q[ii].Transpose().Print(label))
	}
}
func PrintFlux(F []utils.Matrix) {
	for ii := 0; ii < len(F); ii++ {
		label := strconv.Itoa(ii)
		fmt.Println(F[ii].Print("F" + "[" + label + "]"))
	}
}

func nearVec(a, b []float64, tol float64) (l bool) {
	for i, val := range a {
		if !near(b[i], val, tol) {
			fmt.Printf("Diff = %v, Left[%d] = %v, Right[%d] = %v\n", math.Abs(val-b[i]), i, val, i, b[i])
			return false
		}
	}
	return true
}

func nearVecScalar(a []float64, b float64, tol float64) (l bool) {
	for i, val := range a {
		if !near(b, val, tol) {
			fmt.Printf("Diff = %v, Left[%d] = %v, Right[%d] = %v\n", math.Abs(val-b), i, val, i, b)
			return false
		}
	}
	return true
}

func near(a, b float64, tolI ...float64) (l bool) {
	var (
		tol float64
	)
	if len(tolI) == 0 {
		tol = 1.e-08
	} else {
		tol = tolI[0]
	}
	bound := math.Max(tol, tol*math.Abs(a))
	val := math.Abs(a - b)
	if val <= bound {
		l = true
	} else {
		fmt.Printf("Diff = %v, Left = %v, Right = %v\n", val, a, b)
	}
	return
}

func InitializePolynomial(X, Y utils.Matrix) (Q [4]utils.Matrix) {
	var (
		Np, Kmax = X.Dims()
	)
	for n := 0; n < 4; n++ {
		Q[n] = utils.NewMatrix(Np, Kmax)
	}
	for ii := 0; ii < Np*Kmax; ii++ {
		x, y := X.Data()[ii], Y.Data()[ii]
		rho, rhoU, rhoV, E := GetStatePoly(x, y)
		Q[0].Data()[ii] = rho
		Q[1].Data()[ii] = rhoU
		Q[2].Data()[ii] = rhoV
		Q[3].Data()[ii] = E
	}
	return
}

func GetStatePoly(x, y float64) (rho, rhoU, rhoV, E float64) {
	/*
		Matlab script:
				syms a b c d x y gamma
				%2D Polynomial field
				rho=a*abs(x)+b*abs(y);
				u = c*x; v = d*y;
				rhou=rho*u; rhov=rho*v;
				p=rho^gamma;
				q=0.5*rho*(u^2+v^2);
				E=p/(gamma-1)+q;
				U = [ rho, rhou, rhov, E];
				F = [ rhou, rho*u^2+p, rho*u*v, u*(E+p) ];
				G = [ rhov, rho*u*v, rho*v^2+p, v*(E+p) ];
				div = diff(F,x)+diff(G,y);
				fprintf('Code for Divergence of F and G Fluxes\n%s\n',ccode(div));
				fprintf('Code for U \n%s\n%s\n%s\n%s\n',ccode(U));
	*/
	var (
		a, b, c, d = 1., 1., 1., 1.
		pow        = math.Pow
		fabs       = math.Abs
		gamma      = 1.4
	)
	rho = a*fabs(x) + b*fabs(y)
	rhoU = c * x * (a*fabs(x) + b*fabs(y))
	rhoV = d * y * (a*fabs(x) + b*fabs(y))
	E = ((c*c)*(x*x)+(d*d)*(y*y))*((a*fabs(x))/2.0+(b*fabs(y))/2.0) + pow(a*fabs(x)+b*fabs(y), gamma)/(gamma-1.0)
	return
}
func GetDivergencePoly(t, x, y float64) (div [4]float64) {
	var (
		gamma      = 1.4
		pow        = math.Pow
		fabs       = math.Abs
		a, b, c, d = 1., 1., 1., 1.
	)
	div[0] = c*(a*fabs(x)+b*fabs(y)) + d*(a*fabs(x)+b*fabs(y)) + a*c*x*(x/fabs(x)) + b*d*y*(y/fabs(y))
	div[1] = (c*c)*x*(a*fabs(x)+b*fabs(y))*2.0 + c*d*x*(a*fabs(x)+b*fabs(y)) + a*(c*c)*(x*x)*(x/fabs(x)) + a*gamma*(x/fabs(x))*pow(a*fabs(x)+b*fabs(y), gamma-1.0) + b*c*d*x*y*(y/fabs(y))
	div[2] = (d*d)*y*(a*fabs(x)+b*fabs(y))*2.0 + c*d*y*(a*fabs(x)+b*fabs(y)) + b*(d*d)*(y*y)*(y/fabs(y)) + b*gamma*(y/fabs(y))*pow(a*fabs(x)+b*fabs(y), gamma-1.0) + a*c*d*x*y*(x/fabs(x))
	div[3] = c*(((c*c)*(x*x)+(d*d)*(y*y))*((a*fabs(x))/2.0+(b*fabs(y))/2.0)+pow(a*fabs(x)+b*fabs(y), gamma)+pow(a*fabs(x)+b*fabs(y), gamma)/(gamma-1.0)) + d*(((c*c)*(x*x)+(d*d)*(y*y))*((a*fabs(x))/2.0+(b*fabs(y))/2.0)+pow(a*fabs(x)+b*fabs(y), gamma)+pow(a*fabs(x)+b*fabs(y), gamma)/(gamma-1.0)) + c*x*((c*c)*x*((a*fabs(x))/2.0+(b*fabs(y))/2.0)*2.0+(a*(x/fabs(x))*((c*c)*(x*x)+(d*d)*(y*y)))/2.0+a*gamma*(x/fabs(x))*pow(a*fabs(x)+b*fabs(y), gamma-1.0)+(a*gamma*(x/fabs(x))*pow(a*fabs(x)+b*fabs(y), gamma-1.0))/(gamma-1.0)) + d*y*((b*(y/fabs(y))*((c*c)*(x*x)+(d*d)*(y*y)))/2.0+(d*d)*y*((a*fabs(x))/2.0+(b*fabs(y))/2.0)*2.0+b*gamma*(y/fabs(y))*pow(a*fabs(x)+b*fabs(y), gamma-1.0)+(b*gamma*(y/fabs(y))*pow(a*fabs(x)+b*fabs(y), gamma-1.0))/(gamma-1.0))
	return
}

func FluxCalcMomentumOnly(Gamma, rho, rhoU, rhoV, E float64) (Fx, Fy [4]float64) {
	Fx, Fy =
		[4]float64{rhoU, rhoU, rhoU, rhoU},
		[4]float64{rhoV, rhoV, rhoV, rhoV}
	return
}

func CheckFlux0(c *Euler, t *testing.T) {
	/*
	   		Conditions of this test:
	            - Two duplicated triangles, removes the question of transformation Jacobian making the results differ
	            - Flux is calculated identically for each equation (only density components), removes the question of flux
	              accuracy being different for the more complex equations
	            - Flowfield is initialized to a freestream for a polynomial field, interpolation to edges is not done,
	              instead, analytic initialization values are put into the edges
	             Result:
	            - No test of different triangle shapes and orientations
	            - No test of accuracy of interpolation to edges
	            - No accuracy test of the complex polynomial fluxes in Q[1-3]
	*/
	c.FluxCalcMock = FluxCalcMomentumOnly // For testing, only consider the first component of flux for all [4]
	// Initialize
	X, Y := c.dfr.FluxX, c.dfr.FluxY
	QFlux := InitializePolynomial(X, Y)
	Kmax := c.dfr.K
	Nint := c.dfr.FluxElement.Nint
	Nedge := c.dfr.FluxElement.Nedge
	for n := 0; n < 4; n++ {
		for k := 0; k < Kmax; k++ {
			for i := 0; i < Nint; i++ {
				ind := k + i*Kmax
				c.Q[n].Data()[ind] = QFlux[n].Data()[ind]
			}
			for i := 0; i < 3*Nedge; i++ {
				ind := k + i*Kmax
				ind2 := k + (i+2*Nint)*Kmax
				c.Q_Face[n].Data()[ind] = QFlux[n].Data()[ind2]
			}
		}
	}
	c.SetNormalFluxInternal(c.Q)
	// No need to interpolate to the edges, they are left at initialized state in Q_Face
	c.SetNormalFluxOnEdges(0)

	var div utils.Matrix
	for n := 0; n < 4; n++ {
		div = c.dfr.FluxElement.DivInt.Mul(c.F_RT_DOF[n])
		d1, d2 := div.Dims()
		assert.Equal(t, d1, Nint)
		assert.Equal(t, d2, Kmax)
		for k := 0; k < Kmax; k++ {
			_, _, Jdet := c.dfr.GetJacobian(k)
			for i := 0; i < Nint; i++ {
				ind := k + i*Kmax
				div.Data()[ind] /= Jdet
			}
		}
		// Get the analytic values of divergence for comparison
		nn := 0 // Use only the density component of divergence to check
		for k := 0; k < Kmax; k++ {
			for i := 0; i < Nint; i++ {
				ind := k + i*Kmax
				x, y := X.Data()[ind], Y.Data()[ind]
				divC := GetDivergencePoly(0, x, y)
				divCalc := div.Data()[ind]
				normalizer := c.Q[nn].Data()[ind]
				test := near(divCalc/normalizer, divC[nn]/normalizer, 0.0001) // 1% of field value
				if !test {
					fmt.Printf("div[%d][%d,%d] = %8.5f\n", n, k, i, divCalc)
				}
				assert.True(t, test) // 1% of field value
			}
		}
	}
}

func (c *Euler) TestSetNormalFluxOnEdges() {
	var (
		Nedge = c.dfr.FluxElement.Nedge
	)
	edgeFlux := make([][2][4]float64, Nedge)
	for en, e := range c.dfr.Tris.Edges {
		for conn := 0; conn < int(e.NumConnectedTris); conn++ {
			var (
				k          = int(e.ConnectedTris[conn]) // Single tri
				edgeNumber = int(e.ConnectedTriEdgeNumber[conn])
				shift      = edgeNumber * Nedge
			)
			for i := 0; i < Nedge; i++ {
				ie := i + shift
				edgeFlux[i][0], edgeFlux[i][1] = c.CalculateFlux(k, ie, c.Q_Face)
			}
			c.ProjectFluxToEdge(edgeFlux, e, en, conn)
		}
	}
	return
}
