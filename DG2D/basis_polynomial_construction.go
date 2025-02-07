package DG2D

type BasisVectorStruct struct {
	Eval       func(r, s float64) (v [2]float64)
	Dot        func(r, s float64, f [2]float64) (dot float64)
	Project    func(r, s float64, psi float64) (v [2]float64) // scalar mult psi
	Divergence func(r, s float64) (div float64)               // Div of vector
	Sum        func(r, s float64) (sum float64)               // Sum of vector
}

type BasisPolynomialMultiplier struct {
	// This multiplies the BasisVector to produce a term
	Eval        func(r, s float64) (val float64)
	Divergence  func(r, s float64) (div float64)
	OrderOfTerm int
}
type BasisPolynomialTerm struct {
	IsScaled       bool // After scaling, is part of the element polynomial
	PolyMultiplier BasisPolynomialMultiplier
	BasisVector    BasisVectorStruct
}

func (pt BasisPolynomialTerm) Eval(r, s float64) (v [2]float64) {
	v = pt.BasisVector.Project(r, s, pt.PolyMultiplier.Eval(r, s))
	return
}
func (pt BasisPolynomialTerm) Dot(r, s float64, b [2]float64) (dot float64) {
	v := pt.Eval(r, s)
	dot = b[0]*v[0] + b[1]*v[1]
	return
}

func (pt BasisPolynomialTerm) Divergence(r, s float64) (div float64) {
	var (
		divBasis = pt.BasisVector.Divergence(r, s)
		divPoly  = pt.PolyMultiplier.Divergence(r, s)
		polyEval = pt.PolyMultiplier.Eval(r, s)
		sumBasis = pt.BasisVector.Sum(r, s)
	)
	div = polyEval*divBasis + divPoly*sumBasis
	return
}
