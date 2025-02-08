package DG2D

import (
	"fmt"
	"math"
	"testing"

	"github.com/notargets/gocfd/utils"

	utils2 "github.com/notargets/avs/utils"

	"github.com/notargets/avs/chart2d"
	graphics2D "github.com/notargets/avs/geometry"

	"github.com/stretchr/testify/assert"
)

func TestRTElementRTInterpolation(t *testing.T) {
	// Verify the interpolation of a constant vector field onto the element
	// P := 1

	for P := 1; P <= 2; P++ {
		var (
			dt DivTest
		)
		dt = PolyField{}

		rt := NewRTElement(P)
		// rt.V.Print("V")
		// rt.VInv.Print("VInv")
		s1, s2 := make([]float64, rt.Np), make([]float64, rt.Np)
		for PField := 0; PField <= P; PField++ {
			for i := 0; i < rt.Np; i++ {
				r, s := rt.R.AtVec(i), rt.S.AtVec(i)
				f1, f2 := dt.F(r, s, PField)
				s1[i], s2[i] = f1, f2
			}
			rt.ProjectFunctionOntoDOF(s1, s2)

			C := rt.VInv.Mul(rt.Projection)

			// For each polynomial evaluation at (r,s)i
			f_rt_dot := make([]float64, rt.Np)
			for i := 0; i < rt.Np; i++ {
				r_i, s_i := rt.R.AtVec(i), rt.S.AtVec(i)
				b_i := rt.Phi[i].BasisVector.Eval(r_i, s_i)
				// Sum of the basis polynomials over j, each dotted with basis vector_i
				for j := 0; j < rt.Np; j++ {
					f_rt_dot[i] += rt.Phi[j].Dot(r_i, s_i, b_i) * C.At(j, 0)
				}
				if PField >= P+1 {
					r, s := rt.R.AtVec(i), rt.S.AtVec(i)
					fmt.Printf("f_rt[%f,%f]=%f, f_proj=%f\n",
						r, s, f_rt_dot[i], rt.Projection.At(i, 0))
				}
			}
			assert.InDeltaSlicef(t, rt.Projection.DataP, f_rt_dot, 0.000001,
				"Interpolation Check")
		}
	}
}

func TestRTElementDivergence(t *testing.T) {
	// We test RT1 first in isolation because RT1:
	// - uses only the analytic interior basis functions E4 and E5
	// - uses the Lagrange 1D polynomial on edges
	// - is the simplest construction to test divergence
	var (
		dt DivTest
	)
	dt = PolyField{}

	fmt.Println("Begin Divergence Test")
	// P := 1
	PStart := 1
	PEnd := 2
	for P := PStart; P <= PEnd; P++ {
		fmt.Printf("---------------------------------------------\n")
		fmt.Printf("Checking Divergence for RT%d\n", P)
		fmt.Printf("---------------------------------------------\n")
		rt := NewRTElement(P)
		// if P == 2 {
		// 	rt.V.Print("V RT2")
		// 	rt.VInv.Print("VInv RT2")
		// 	rt.Div.Print("Div RT2")
		// }
		Np := rt.Np
		divFcalc := make([]float64, Np)
		s1, s2 := make([]float64, Np), make([]float64, Np)
		// for PField := 0; PField <= (P - 1); PField++ {
		for PField := 0; PField <= P; PField++ {
			fmt.Printf("\nReference Vector Field Order:%d\n", PField)
			fmt.Printf("-------------------------------\n")
			for i := 0; i < Np; i++ {
				r, s := rt.R.AtVec(i), rt.S.AtVec(i)
				f1, f2 := dt.F(r, s, PField)
				s1[i], s2[i] = f1, f2
				dF := dt.divF(r, s, PField)
				divFcalc[i] = dF
			}
			dFReference := utils.NewMatrix(Np, 1, divFcalc)
			dFReference.Transpose().Print("Reference Div")
			// if PField == 1 {
			// 	for i := 0; i < Np; i++ {
			// 		r, s := rt.R.AtVec(i), rt.S.AtVec(i)
			// 		fmt.Printf("f[%f,%f] = [%f,%f] \n", r, s, s1[i], s2[i])
			// 	}
			// 	os.Exit(1)
			// }
			rt.ProjectFunctionOntoDOF(s1, s2)
			dB := rt.Projection
			// dB.Transpose().Print("F Projection")
			// rt.VInv.Mul(dB).Print("Coefficients")
			calcDiv := rt.Div.Mul(dB)
			calcDiv.Transpose().Print("Calculated Divergence")
			assert.InDeltaSlice(t, dFReference.DataP, calcDiv.DataP, 0.0001)
		}
	}
}

func TestRTElement(t *testing.T) {
	{
		// Check term-wise orthogonal 2D polynomial basis
		N := 2
		R, S := NodesEpsilon(N - 1)
		JB2D := NewJacobiBasis2D(N-1, R, S, 0, 0)
		ii, jj := 1, 1
		p := JB2D.Simplex2DP(R, S, ii, jj)
		ddr, dds := JB2D.GradSimplex2DP(R, S, ii, jj)
		Np := R.Len()
		pCheck, ddrCheck, ddsCheck := make([]float64, Np), make([]float64, Np), make([]float64, Np)
		for i, rVal := range R.DataP {
			sVal := S.DataP[i]
			ddrCheck[i] = JB2D.PolynomialTermDr(rVal, sVal, ii, jj)
			ddsCheck[i] = JB2D.PolynomialTermDs(rVal, sVal, ii, jj)
			pCheck[i] = JB2D.PolynomialTerm(rVal, sVal, ii, jj)
		}
		assert.True(t, nearVec(pCheck, p, 0.000001))
		assert.True(t, nearVec(ddrCheck, ddr, 0.000001))
		assert.True(t, nearVec(ddsCheck, dds, 0.000001))
	}
	errorCheck := func(N int, div, divCheck []float64) (minInt, maxInt, minEdge, maxEdge float64) {
		var (
			Npm    = len(div)
			errors = make([]float64, Npm)
		)
		for i := 0; i < Npm; i++ {
			// var ddr, dds float64
			errors[i] = div[i] - divCheck[i]
		}
		minInt, maxInt = errors[0], errors[0]
		Nint := N * (N + 1) / 2
		minEdge, maxEdge = errors[Nint], errors[Nint]
		for i := 0; i < Nint; i++ {
			errAbs := math.Abs(errors[i])
			if minInt > errAbs {
				minInt = errAbs
			}
			if maxInt < errAbs {
				maxInt = errAbs
			}
		}
		for i := Nint; i < Npm; i++ {
			errAbs := math.Abs(errors[i])
			if minEdge > errAbs {
				minEdge = errAbs
			}
			if maxEdge < errAbs {
				maxEdge = errAbs
			}
		}
		fmt.Printf("Order = %d, ", N)
		fmt.Printf("Min, Max Int Err = %8.5f, %8.5f, Min, Max Edge Err = %8.5f, %8.5f\n", minInt, maxInt, minEdge, maxEdge)
		return
	}
	checkSolution := func(rt *RTElement, Order int) (s1, s2, divCheck []float64) {
		var (
			Np = rt.Np
		)
		s1, s2 = make([]float64, Np), make([]float64, Np)
		divCheck = make([]float64, Np)
		var ss1, ss2 float64
		for i := 0; i < Np; i++ {
			r := rt.R.DataP[i]
			s := rt.S.DataP[i]
			ccf := float64(Order)
			s1[i] = utils.POW(r, Order)
			s2[i] = utils.POW(s, Order)
			ss1, ss2 = ccf*utils.POW(r, Order-1), ccf*utils.POW(s, Order-1)
			divCheck[i] = ss1 + ss2
		}
		return
	}
	// against analytical solution
	// Nend := 8
	// for N := 1; N < Nend; N++ {
	N := 1
	rt := NewRTElement(N)
	for cOrder := 0; cOrder < N; cOrder++ {
		fmt.Printf("Check Order = %d, ", cOrder)
		// [s1,s2] values for each location in {R,S}
		s1, s2, divCheck := checkSolution(rt, cOrder)
		rt.ProjectFunctionOntoDOF(s1, s2)
		divM := rt.Div.Mul(rt.Projection)
		// fmt.Println(divM.Print("divM"))
		minerrInt, maxerrInt, minerrEdge, maxerrEdge := errorCheck(N, divM.DataP, divCheck)
		assert.True(t, near(minerrInt, 0.0, 0.00001))
		assert.True(t, near(maxerrInt, 0.0, 0.00001))
		assert.True(t, near(minerrEdge, 0.0, 0.00001))
		assert.True(t, near(maxerrEdge, 0.0, 0.00001))
	}
	// }
	plot := false
	if plot {
		N := 2
		rt := NewRTElement(N)
		s1, s2 := make([]float64, rt.R.Len()), make([]float64, rt.R.Len())
		for i := range rt.R.DataP {
			s1[i] = 1
			s2[i] = 1
		}
		if plot {
			chart := PlotTestTri(true)
			points := utils.ArraysToPoints(rt.R.DataP, rt.S.DataP)
			f := utils.ArraysTo2Vector(s1, s2, 0.1)
			_ = chart.AddVectors("test function", points, f, chart2d.Solid, utils.GetColor(utils.Green))
			utils.SleepFor(500000)
		}
	}
}

func PlotTestTri(plotGeom bool) (chart *chart2d.Chart2D) {
	var (
		points  []graphics2D.Point
		trimesh graphics2D.TriMesh
		K       = 1
	)

	points = make([]graphics2D.Point, 3)
	points[0].X[0], points[0].X[1] = -1, -1
	points[1].X[0], points[1].X[1] = 1, -1
	points[2].X[0], points[2].X[1] = -1, 1

	trimesh.Triangles = make([]graphics2D.Triangle, K)
	colorMap := utils2.NewColorMap(0, 1, 1)
	trimesh.Triangles[0].Nodes[0] = 0
	trimesh.Triangles[0].Nodes[1] = 1
	trimesh.Triangles[0].Nodes[2] = 2
	trimesh.Geometry = points
	box := graphics2D.NewBoundingBox(trimesh.GetGeometry())
	chart = chart2d.NewChart2D(1024, 1024, box.XMin[0], box.XMax[0], box.XMin[1], box.XMax[1])
	chart.AddColorMap(colorMap)
	go chart.Plot()

	if plotGeom {
		if err := chart.AddTriMesh("TriMesh", trimesh,
			chart2d.CrossGlyph, 0.1, chart2d.Solid,
			utils.GetColor(utils.Black)); err != nil {
			panic("unable to add graph series")
		}
	}
	return
}

func checkIfUnitMatrix(t *testing.T, A utils.Matrix) (isDiag bool) {
	var (
		Np, _ = A.Dims()
	)
	for j := 0; j < Np; j++ {
		for i := 0; i < Np; i++ {
			if i == j {
				assert.InDeltaf(t, 1., A.At(i, j), 0.00001, "")
			} else {
				assert.InDeltaf(t, 0., A.At(i, j), 0.00001, "")
			}
		}
	}
	return
}

type DivTest interface {
	F(r, s float64, P int) (f1, f2 float64)
	divF(r, s float64, P int) (div float64)
}

type SinCosField struct{}

func (scf SinCosField) F(r, s float64, P int) (f1, f2 float64) {
	var (
		Pi = math.Pi
	)
	conv := func(r float64) (xi float64) {
		xi = Pi * (r + 1)
		return
	}
	f1, f2 = math.Sin(conv(r)), math.Cos(conv(s))
	return
}

func (scf SinCosField) divF(r, s float64, P int) (div float64) {
	var (
		Pi = math.Pi
	)
	conv := func(r float64) (xi float64) {
		xi = Pi * (r + 1)
		return
	}
	div = (math.Cos(conv(r)) - math.Sin(conv(s)))
	div += Pi * (math.Sin(conv(r)) + math.Cos(conv(s)))
	return
}

type PolyField struct{}

func (lpf PolyField) F(r, s float64, P int) (f1, f2 float64) {
	var (
		p = float64(P)
	)
	f1, f2 = math.Pow(r+s+10, p), math.Pow(10*(r+s), p)
	return
}

func (lpf PolyField) divF(r, s float64, P int) (div float64) {
	var (
		p = float64(P)
	)
	if P > 0 {
		div = p * (math.Pow(r+s+10, p-1) + 10*math.Pow(10*(r+s), p-1))
	} else {
		div = 0
	}
	return
}
