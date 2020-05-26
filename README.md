# gocfd
Awesome CFD solver written in Go 

## An implementation of the Discontinuous Galerkin Method for solving systems of equations

### Credits to Jan S. Hesthaven and Tim Warburton for their excellent text "Nodal Discontinuous Galerkin Methods" (2007)

### Guide to code review

If you are interested in reviewing the physics and how it is implemented, look through the code in "model_problems". Each file there implements one physics model or an additional numerical method for a model.

If you want to look at the math / matrix library implementation, take a look at the code in utils/matrix_extended.go and utils/vector_extended.go. I've implemented a chained operator syntax for operations that favors reuse / reduces copying and makes clear the dimensionality of the operands. It is far from perfect and complete, especially WRT value assignment and indexing, as I built the library to emulate and implement what was in the text and matlab, however it is functional / useful. The syntax is somewhat like RPN (reverse polish notation) in that you manage an accumulation of value over chained operations.

For example, the following line implements:
```rhsE = - Dr * FluxH .* Rx ./ epsilon``` Dr is the "Derivative Matrix" and Rx is (1 / J), applying the transform of R to X, and epsilon is the metalic impedance, all applied to the Flux matrix:
```
	RHSE = el.Dr.Mul(FluxH).ElMul(el.Rx).ElDiv(c.Epsilon).Scale(-1)
```

### QuickStart

Using Ubuntu Linux, do the following:
```
### Build
me@home:bash# sudo apt update
me@home:bash# sudo apt install libx11-dev libxi-dev libxcursor-dev libxrandr-dev libxinerama-dev mesa-common-dev libgl1-mesa-dev
me@home:bash# make
```
```
### Run with graphics:
me@home:bash# export DISPLAY=:0
me@home:bash# gocfd -graph
```
```
### Run without graphics:
me@home:bash# gocfd
```
### Updates (May 26, 2000): Fixed the Exact solution to the Sod shock tube
#### T = 0.2, N=3, 2000 Elements
DFR Roe | Galerkin Lax
:-------------------------:|:-------------------------:
![](images/EulerDFR-K2000-N3.PNG) | ![](images/EulerGK-K2000-N3.PNG)

DFR/Roe versus Galerkin/Lax: In the DFR solution the contact discontinuity is steeper than the GK/Lax solution. There is a very slight position error for the contact discontinuity in the DFR solution and also a bump on the left side of it, an artifact of the underdamped aliasing.

#### T = 0.2, N=3, Galerkin Lax Flux, 500 Elements
![](images/EulerGKRho1-fixed.PNG)

The shock speed problem I saw yesterday turns out to have been the exact solution :-)

After correcting the exact solution to Sod's shock tube problem, the two Euler solvers match up pretty well all around, with some small differences between them - phew!

### Update (May 25, 2000): Roe Flux with DFR - Euler 1D compared to Analytic Solution in real time
#### T = 0.223, N=4, Roe Flux, 600 Elements
![](images/EulerDFRRho1.PNG)

This is cool - being able to see exactly the errors and successes in realtime. The above is a snap of an interim result where I'm now showing the exact solution in symbols overlaying the simulation in realtime and sure enough we see a shock speed error on the leading shock wave, along with excellent reproduction of the smooth expansion flow.

I also went back and checked the Galerkin (non-DFR) Euler case and it has the same error in shock propagation speed as the DFR/Roe result, which says there's a common error somewhere. It's good to spend time doing basic accuracy tests!

You can recreate this using ``` gocfd -graph -model 5 -CFL 0.75 -N 4 -K 600```

### Update (May 12, 2020): DFR and Aliasing, Instability and Fixing it
On the path to implementing direct flux reconstruction, I found what appeared to be 2nd order aliasing without a clear origin. After consulting Hesthaven(2007) section 5.3, I see that the issue is the interpolation of the flux and subsequently taking the derivative of that interpolated flux. As stated: "the derivative of the interpolation is not the same as the interpolation of the derivative", or put another way, by simply computing the flux from the nodal points of the solution polynomial, we are not treating the flux formally as a polynomial - instead we should perform a formal polynomial fit (projection, instead of interpolation) of the flux prior to using that polynomial to compute derivatives of the flux. The result of using interpolation shown in the text is that we produce an aliasing error into the solution, and their answer is to filter it away instead of using the much more compute intensive projection. The aliasing error also gets worse with increasing N, so the filter should change with N.

I've implemented a simple 2nd order dissipative filter with a constant coefficient, which works well to knock out the oscillations without introducing solution error. I also experimented with a combined 2nd / 4th order filter, similar to what is commonly used in finite volume schemes and for this problem the 2nd order dissipation was enough. However, I can not get stable results beyond about N=6 with this filter, so I'll also investigate an efficient way to do projection and/or improved filters.

--> prior updates
DFR for Maxwell's equations now uses 2nd order artificial dissipation to remove the odd/even aliasing and it works pretty well, which affirms my earlier finding of higher order modes. Next steps might include doing a stability analysis on the DFR scheme, covering the details of the reconstruction. It is slightly different than in Jameson(2014), in that this formulation of NDG uses N+1 points for the polynomials, including the node edges. In Jameson(2014), they extended the flux points to N+3 to cover the element edges, where here I just formed the fluxes at the face points, which are at i=1 and i=N+1. 

You can see it as model 4 with default parameters like ```gocfd -model 4 -graph``` The issue is the instability - it scales with N, so is a higher order modal signal - It could be that the Lax Friedrich's flux, which is 1st order, is not providing damping of the higher order modes. But why are there higher order modes? Are these unresolved waves? I can't answer without more looking into it...

### Current Status (May 9, 2020): Direct Flux Reconstruction implemented for 1D Advection, moving to implement for the other two 1D model problems

DFR works for Advection (-model 3) and seems to improve accuracy and physicality.

The primary differences between using DFR and traditional Nodal Discontinuous Galerkin:
1) Instead of using only the primitive variables in the right hand side, we use the Flux directly for a hyperbolic problem
2) The flux is a globally composed variable and is differentiated after being "reconstructed" to be C(0)
3) The reconstruction technique follows Jameson(2014) in that we coerce the flux values of each face to be consistent values at each face, then use the same Np Gauss-Lobato nodes and metrics to develop the derivative(s) of flux

For (3) in this case, I used the same Lax-Friedrichs flux calculation from the text, then averaged the values from each side of each face together to make a single consistent face value shared by neighboring elements.

### (May 1, 2020): Researching Direct Flux Reconstruction methods
During testing of the method in 1D as outlined in the text, it became clear that the slope limiter is quite crude and is degrading the physicality of the solution. The authors were clear that this is just for example use, and now I'm convinced of it!

My first round of research yielded the [Direct Flux Reconstruction](research/filters_and_flux_limiters/Romero2015.pdf) technique from a 2014 paper by Antony Jameson, et al (one of my favorite CFD people of all time). The technique is extremely simple and has the great promise of extending the degree of accuracy to the Flux terms of more complex equations, in addition to enabling the use of flux limiters that have been proven for flows with discontinuities.

In my past CFD experience, discontinuities in solving the Navier Stokes equations are not limited to shock waves. Rather, we find shear flows have discontinuities that are similar enough to shock waves that shock finding techniques used to guide the application of numerical diffusion can be active at the edge of boundary layers and at other critical regions, which often leads to inaccuracies far from shocks. The text's gradient based limiter would clearly suffer from this kind of problem among others.

## Requirements to run the code
Here is what I'm using as a platform:
```
me@home:bash# go version
go version go1.14 linux/amd64
me@home:bash# cat /etc/os-release
NAME="Ubuntu"
VERSION="18.04.4 LTS (Bionic Beaver)"
...
```
You also need to install some X11 and OpenGL related packages in Ubuntu, like this:
```
apt update
apt install libx11-dev libxi-dev libxcursor-dev libxrandr-dev libxinerama-dev mesa-common-dev libgl1-mesa-dev
```
A proper build should go like this:
```
me@home:bash# make
go fmt ./...  && go install ./...
run this -> $GOPATH/bin/gocfd
me@home:bash# /gocfd$ gocfd --help
Usage of gocfd:
  -K int
        Number of elements in model (default 60)
  -N int
        polynomial degree (default 8)
  -delay int
        milliseconds of delay for plotting (default 0)
  -graph
        display a graph while computing solution
  -model int
        model to run: 0 = Advect1D, 1 = Maxwell1D (default 1)
```
### Current Status: Researching Direct Flux Reconstruction methods

During testing of the method in 1D as outlined in the text, it became clear that the slope limiter is quite crude and is degrading the physicality of the solution. The authors were clear that this is just for example use, and now I'm convinced of it!

My first round of research yielded the [Direct Flux Reconstruction](research/filters_and_flux_limiters/Romero2015.pdf) technique from a 2014 paper by Antony Jameson, et al (one of my favorite CFD people of all time). The technique is extremely simple and has the great promise of extending the degree of accuracy to the Flux terms of more complex equations, in addition to enabling the use of flux limiters that have been proven for flows with discontinuities.

In my past CFD experience, discontinuities in solving the Navier Stokes equations are not limited to shock waves. Rather, we find shear flows have discontinuities that are similar enough to shock waves that shock finding techniques used to guide the application of numerical diffusion can be active at the edge of boundary layers and at other critical regions, which often leads to inaccuracies far from shocks. The text's gradient based limiter would clearly suffer from this kind of problem among others.

### Model Problem Example #3a: Euler's Equations in 1D - Shock Collision

This is an interesting problem because of the temperature remainder after the collision. In the plot, temperature is red, density is blue, and velocity is orange. After the shocks pass out of the domain, the remaining temperature "bubble" can't dissipate, because the Euler equations have no mechanism for temperature diffusion.

This case is obtained by initializing the tube as with the Sod tube, but leaving the exit boundary at the left side values (In == Out). This produces a left running shock wave that meets with the shock moving right.

#### T = 0.36, 1000 Elements
![](images/Euler1D-MidTube-K1000-N6-T.36.PNG)

### Model Problem Example #3: Euler's Equations in 1D - Sod's shock tube

The 1D Euler equations are solved with boundary and initial conditions for the Sod shock tube problem. There is an analytic solution for this case and it is widely used to test shock capturing ability of a solver.

Run the example with graphics like this:
```
bash# make
bash# gocfd -model 2 -graph -K 250 -N 1
```

You can also target a final time for the simulation using the "-FinalTime" flag. You will have to use CTRL-C to exit the simulation when it arrives at the target time. This leaves the plot on screen so you can screen cap it.
```
bash# gocfd -model 2 -graph -K 250 -N 1 -FinalTime 0.2
```
#### T = 0.2, 60 Elements
Linear Elements | 10th Order Elements
:-------------------------:|:-------------------------:
![](images/Euler1D-SOD-K60-N1-T0.2.PNG) | ![](images/Euler1D-SOD-K60-N10-T0.2.PNG)

#### T = 0.2, 250 Elements
Linear Elements | 10th Order Elements
:-------------------------:|:-------------------------:
![](images/Euler1D-SOD-K250-N1-T0.2.PNG) | ![](images/Euler1D-SOD-K250-N10-T0.2.PNG)

#### T = 0.2, 500 Elements
Linear Elements | 10th Order Elements
:-------------------------:|:-------------------------:
![](images/Euler1D-SOD-K500-N1-T0.2.PNG) | ![](images/Euler1D-SOD-K500-N10-T0.2.PNG)


### Model Problem Example #2: Maxwell's Equations solved in a 1D Cavity

The Maxwell equations are solved in a 1D metal cavity with a change of material half way through the domain. The initial condition is a sine wave for the E (electric) field in the left half of the domain, and zero for E and H everywhere else. The E field is zero on the boundary (face flux out = face flux in) and the H field passes through unchanged (face flux zero), corresponding to a metallic boundary.



Run the example with graphics like this:
```
bash# make
bash# gocfd -model 1 -delay 0 -graph -K 80 -N 5
```

Unlike the advection equation model problem, this solver does have unstable points in the space of K (element count) and N (polynomial degree). So far, it appears that the polynomial degree must be >= 5 for stability, otherwise aliasing occurs, where even/odd modes are excited among grid points.

In the example pictured, there are 80 elements (K=80) and the element polynomial degree is 5 (N=5).

#### Initial State
![](images/Maxwell1D-cavity0.PNG)

#### Intermediate State
![](images/Maxwell1D-cavity.PNG)

#### First Mode
![](images/Maxwell1D-cavity2.PNG)

#### Second Mode
![](images/Maxwell1D-cavity4.PNG)

### Model Problem Example #1: Advection Equation
<span style="display:block;text-align:center">![](images/Advect1D-0.PNG)</span>

The first model problem is 1D Advection with a left boundary driven sine wave. You can run it with integrated graphics like this:
```
bash# gocfd -model 0 -delay 0 -graph -K 80 -N 5
```

In the example pictured, there are 80 elements (K=80) and the element polynomial degree is 5 (N=5).

<span style="display:block;text-align:center">![](images/Advect1D-1.PNG)</span>
