package DG2D

import "github.com/notargets/gocfd/utils"

type DFR2D struct {
	N, Np        int
	R, S         utils.Vector // Solution point locations in unit triangle
	FluxR, FluxS utils.Vector // Flux (face) point locations in unit triangle
	V, Vinv      utils.Matrix // Vandermonde and inverse for solution polynomial (order N), defined on solution points
	Dr, Ds       utils.Matrix // Differentiation matrices in R,S directions for solution points
}

func NewDFR2D(N int) (dfr *DFR2D) {
	var (
		err error
	)
	dfr = &DFR2D{
		N:  N,
		Np: (N + 1) * (N + 2) / 2,
	}
	// Compute nodal set
	dfr.R, dfr.S = NodesEpsilon(N)
	// Build reference element matrices
	dfr.V = Vandermonde2D(N, dfr.R, dfr.S)
	if dfr.Vinv, err = dfr.V.Inverse(); err != nil {
		panic(err)
	}
	// Initialize the (r,s) differentiation matrices on the simplex, evaluated at (r,s) at order N
	Vr, Vs := GradVandermonde2D(N, dfr.R, dfr.S)
	dfr.Dr = Vr.Mul(dfr.Vinv)
	dfr.Ds = Vs.Mul(dfr.Vinv)

	// Mark fields read only
	dfr.Dr.SetReadOnly("Dr")
	dfr.Ds.SetReadOnly("Ds")
	dfr.V.SetReadOnly("V")
	dfr.Vinv.SetReadOnly("Vinv")
	return
}

// Build reference element matrices
/*
		We build the mixed elements for the DFR scheme with:

		Solution Points: We use points within a reference triangle, excluding the edges, for a Lagrangian element
		of O(K) to store the solution. If we need derivatives, or interpolated quantities (Flux), we use the
		solution points element.

		Flux Points: We use a customized Raviart-Thomas (RT) vector element of O(K+1) to store the vector Flux function
	    computed from the solution values. The RT element is of order O(K+1) and is a combination of the points from
		the solution element for the interior, and points along the three triangle edges. The custom RT basis is
		established using a procedure outlined in: "Ainv Direct Flux Reconstruction Scheme for Advection-Diffusion
		Problems on Triangular Grids" by Romero, Witherden and Jameson (2017). Ainv complete RT basis, [ B ], is used
		together with unit basis vectors, [ w ], to satisfy the following:
				[ B_j(r_i) dot w_i ] [ C ] = [ delta_i_j ]
				=> solve for [ C ], the coefficients defining the custom RT basis

		[ C ] is the vector of coefficients defining the basis using the basis vectors [ w ] and [ B ].

		The [ w ] directions of the custom RT element basis are defined such that:
			w([r]) = w(edge_locations) = unit normals on each of three edges
			w([r]) = w(interior) = unit normals in the two primary geometry2D directions (r and s)

		For order K there are:
			- (K+1) locations on each edge, for a total of 3(K+1) edge basis functions.
			- (K)(K+1) locations in the interior, half for the w_r direction and half for the w_s direction
			- Total: (K+3)(K+1) basis functions for the custom RT_K element

		Notes:
			1) The number of interior points matches the Lagrangian element in 2D at order (K-1). Ainv Lagrange element
			at order (K) has N_p = (K+1)(K+2)/2 degrees of freedom, so an order (K-1) element has (K)(K+1)/2 DOF.
			Considering that we need a term for each of the two interior directions at each interior point, we need
			exactly 2*N_p DOF at order (K-1) for the interior of the custom RT element, resulting in (K)(K+1) terms.
			2) Note (1) confirms that the custom element requires exactly the same number of interior points
			(K)(K+1)/2 as a Lagrange element of order (K-1), which means we can use the custom RT element for the
			DFR approach, which needs to provide a O(K+1) element to preserve the gradient at O(K). We will use the
			solution points from the Lagrange element at O(K) to construct the interior of the O(K+1) RT element
			without requiring interpolation of the solution points, as they already reside at the same geometric
			locations.
			(3) To create the custom RT element, we initialize the solution element, then define the custom RT element
			from the interior point locations of the solution element to ensure that they are colocated.
			(4) To use the custom RT element:
			a) calculate the solution, calculate the flux vector field from the solution at the solution points
			b) transfer the flux vector field values to the DFR element interior
			c) interpolate flux values at from the interior of the RT element to the locations on the triangle edges
			d) use the method of characteristics to calculate the corrected flux using the neighbor element's edge
			flux combined with the edge flux from this element
			e) calculate the gradient of the vector flux field using the custom RT element
			f) transfer the gradient values from the RT element to the solution element for use in advancing the
			solution in differential form (directly)

		By calculating the flux gradient in a way that yields an O(K) polynomial on the solution points, we can use
		the differential form of the equations directly for the solution, rather than using the traditional Galerkin
		approach of repeated integration by parts to obtain an equation with only first derivatives. This simplifies
		the solution process, resulting in a more efficient computational approach, in addition to making it easier
		to solve more complex equations with the identical formulation.
*/
