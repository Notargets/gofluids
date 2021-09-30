package Euler2D

import (
	"fmt"
	"math"

	"github.com/notargets/gocfd/utils"
)

type FlowFunction uint8

func (pm FlowFunction) String() string {
	strings := []string{
		"Density",
		"XMomentum",
		"YMomentum",
		"Energy",
		"Mach",
		"Static Pressure",
		"Dynamic Pressure",
		"Pressure Coefficient",
		"Sound Speed",
		"Velocity",
		"XVelocity",
		"YVelocity",
		"Enthalpy",
		"Entropy",
	}
	switch {
	case int(pm) < len(strings):
		return strings[int(pm)]
	case pm == ShockFunction:
		return "ShockFunction"
	case pm == EpsilonDissipation:
		return "Artificial Dissipation Epsilon"
	default:
		return "Unknown"
	}
}

const (
	Density FlowFunction = iota
	XMomentum
	YMomentum
	Energy
	Mach                // 4
	StaticPressure      // 5
	DynamicPressure     // 6
	PressureCoefficient // 7
	SoundSpeed          // 8
	Velocity            // 9
	XVelocity           // 10
	YVelocity           // 11
	Enthalpy            // 12
	Entropy             //13
	ShockFunction       = 100
	EpsilonDissipation  = 101
)

type FreeStream struct {
	Gamma             float64
	Qinf              [4]float64
	Pinf, QQinf, Cinf float64
	Alpha             float64
	Minf              float64
}

func NewFreeStream(Minf, Gamma, Alpha float64) (fs *FreeStream) {
	var (
		ooggm1 = 1. / (Gamma * (Gamma - 1.))
		uinf   = Minf * math.Cos(Alpha*math.Pi/180.)
		vinf   = Minf * math.Sin(Alpha*math.Pi/180.)
	)
	fs = &FreeStream{
		Gamma: Gamma,
		Qinf:  [4]float64{1, uinf, vinf, ooggm1 + 0.5*Minf*Minf},
		Alpha: Alpha,
		Minf:  Minf,
	}
	fs.Pinf = fs.GetFlowFunctionQQ(fs.Qinf, StaticPressure)
	fs.QQinf = fs.GetFlowFunctionQQ(fs.Qinf, DynamicPressure)
	fs.Cinf = fs.GetFlowFunctionQQ(fs.Qinf, SoundSpeed)
	return
}

func (fs *FreeStream) Print() string {
	return fmt.Sprintf("Minf[%5.2f] Gamma[%5.2f] Alpha[%5.2f] Q[%8.5f,%8.5f,%8.5f,%8.5f]\n",
		fs.Minf, fs.Gamma, fs.Alpha,
		fs.Qinf[0], fs.Qinf[1], fs.Qinf[2], fs.Qinf[3])
}

func (fs *FreeStream) GetFlowFunction(Q [4]utils.Matrix, ind int, pf FlowFunction) (f float64) {
	return fs.GetFlowFunctionBase(Q[0].DataP[ind], Q[1].DataP[ind], Q[2].DataP[ind], Q[3].DataP[ind], pf)
}

func (fs *FreeStream) GetFlowFunctionQQ(Q [4]float64, pf FlowFunction) (f float64) {
	return fs.GetFlowFunctionBase(Q[0], Q[1], Q[2], Q[3], pf)
}

func (fs *FreeStream) GetFlowFunctionBase(rho, rhoU, rhoV, E float64, pf FlowFunction) (f float64) {
	var (
		Gamma = fs.Gamma
		GM1   = Gamma - 1.
		oorho = 1. / rho
		q, p  float64
	)
	// Calculate q if needed
	switch pf {
	case StaticPressure, PressureCoefficient, SoundSpeed, Entropy:
		q = 0.5 * (rhoU*rhoU + rhoV*rhoV) * oorho
	}
	// Calculate p if needed
	switch pf {
	case PressureCoefficient, SoundSpeed, Enthalpy, Mach, Entropy:
		p = GM1 * (E - q)
	}

	switch pf {
	case Density:
		f = rho
	case XMomentum:
		f = rhoU
	case YMomentum:
		f = rhoV
	case Energy:
		f = E
	case StaticPressure:
		f = GM1 * (E - q)
	case DynamicPressure:
		f = 0.5 * (rhoU*rhoU + rhoV*rhoV) * oorho
	case PressureCoefficient:
		f = (p - fs.Pinf) / fs.QQinf
	case SoundSpeed:
		f = math.Sqrt(math.Abs(Gamma * p * oorho))
	case Velocity:
		f = math.Sqrt((rhoU*rhoU + rhoV*rhoV) * oorho)
	case XVelocity:
		f = rhoU * oorho
	case YVelocity:
		f = rhoV * oorho
	case Mach:
		C := math.Sqrt(math.Abs(Gamma * p * oorho))
		U := math.Sqrt((rhoU*rhoU + rhoV*rhoV)) * oorho
		f = U / C
	case Enthalpy:
		f = (E + p) / rho
	case Entropy:
		f = math.Log(p) - Gamma*math.Log(rho)
	}
	return
}
