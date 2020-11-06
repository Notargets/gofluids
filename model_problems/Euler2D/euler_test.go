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
			c := NewEuler(1, 1, N, "../../DG2D/test_tris_5.neu", FLUX_Average, FREESTREAM, false)
			K := c.dfr.K
			Nint := c.dfr.FluxElement.Nint
			Nedge := c.dfr.FluxElement.Nedge
			for n := 0; n < 4; n++ {
				for i := 0; i < Nint; i++ {
					for k := 0; k < K; k++ {
						c.Q[n].Data()[k+i*K] = float64(k + 1)
					}
				}
			}
			// Interpolate from solution points to edges using precomputed interpolation matrix
			for n := 0; n < 4; n++ {
				c.Q_Face[n] = c.dfr.FluxInterpMatrix.Mul(c.Q[n])
			}
			for n := 0; n < 4; n++ {
				for i := 0; i < 3*Nedge; i++ {
					for k := 0; k < K; k++ {
						assert.True(t, near(float64(k+1), c.Q_Face[n].Data()[k+i*K], 0.000001))
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
			c := NewEuler(1, 1, N, "../../DG2D/test_tris_5.neu", FLUX_Average, FREESTREAM, false)
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
				c.Q_Face[n] = c.dfr.FluxInterpMatrix.Mul(c.Q[n])
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
			c := NewEuler(1, 1, N, "../../DG2D/test_tris_5.neu", FLUX_Average, FREESTREAM, false)
			Kmax := c.dfr.K
			Nint := c.dfr.FluxElement.Nint
			c.SetNormalFluxInternal()
			c.SetNormalFluxOnEdges()
			// Check that freestream divergence on this mesh is zero
			for n := 0; n < 4; n++ {
				for k := 0; k < Kmax; k++ {
					_, _, Jdet := c.dfr.GetJacobian(k)
					var div utils.Matrix
					div = c.dfr.FluxElement.DivInt.Mul(c.F_RT_DOF[n])
					for i := 0; i < Nint; i++ {
						ind := k + i*Kmax
						div.Data()[ind] *= 1. / Jdet
					}
					assert.True(t, nearVecScalar(div.Data(), 0., 0.000001))
				}
			}
		}
	}
	{ // Test Isentropic Vortex
		N := 1
		c := NewEuler(1, 1, N, "../../DG2D/vortexA04.neu", FLUX_Average, IVORTEX, false)
		PrintQ(c.Q, "IVortex_Q")
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
	if math.Abs(a-b) <= bound {
		l = true
	}
	return
}
