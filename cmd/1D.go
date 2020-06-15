/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"math"
	"time"

	"github.com/notargets/gocfd/DG2D"
	"github.com/notargets/gocfd/model_problems/Advection1D"
	"github.com/notargets/gocfd/model_problems/Euler1D"
	"github.com/notargets/gocfd/model_problems/Maxwell1D"
	"github.com/spf13/cobra"
)

// OneDCmd represents the 1D command
var OneDCmd = &cobra.Command{
	Use:   "1D",
	Short: "One Dimensional Model Problem Solutions",
	Long: `
Executes the Nodal Discontinuous Galerkin solver for a variety of model problems, with
optional live plots of the solutions. For example:

gocfd 1D -graph`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("1D called")
		mr, _ := cmd.Flags().GetInt("model")
		ModelRun = ModelType(mr)
		dr, _ := cmd.Flags().GetInt("delay")
		Delay = time.Duration(dr)
		Casep, _ := cmd.Flags().GetInt("case")
		Case = Euler1D.CaseType(Casep)
		XMax, _ = cmd.Flags().GetFloat64("xMax")
		FinalTime, _ = cmd.Flags().GetFloat64("finalTime")
		CFL, _ = cmd.Flags().GetFloat64("CFL")
		GridFile, _ = cmd.Flags().GetString("gridFile")
		Graph, _ = cmd.Flags().GetBool("graph")
		N, _ = cmd.Flags().GetInt("n")
		K, _ = cmd.Flags().GetInt("k")
		CFL = LimitCFL(ModelRun, CFL)
		Run()
	},
}

func init() {
	rootCmd.AddCommand(OneDCmd)
	var CaseInt int
	CFL, XMax, N, K, CaseInt = Defaults(ModelRun)
	OneDCmd.Flags().IntP("model", "m", int(ModelRun), "model to run: 0 = Advect1D, 1 = Maxwell1D, 2 = Euler1D")
	OneDCmd.Flags().IntP("k", "k", K, "Number of elements in model")
	OneDCmd.Flags().IntP("n", "n", N, "polynomial degree")
	OneDCmd.Flags().IntP("delay", "d", 0, "milliseconds of delay for plotting")
	OneDCmd.Flags().IntP("case", "c", int(CaseInt), "Case to run, for Euler: 0 = SOD Shock Tube, 1 = Density Wave")
	OneDCmd.Flags().BoolP("graph", "g", false, "display a graph while computing solution")
	OneDCmd.Flags().Float64("CFL", CFL, "CFL - increase for speedup, decrease for stability")
	OneDCmd.Flags().Float64("finalTime", FinalTime, "FinalTime - the target end time for the sim")
	OneDCmd.Flags().Float64("xMax", XMax, "Maximum X coordinate (for Euler) - make sure to increase K with XMax")
	OneDCmd.Flags().String("gridfile", "", "Grid file to read in Gambit (.neu) format")
}

var (
	K         = 0 // Number of elements
	N         = 0 // Polynomial degree
	Delay     = time.Duration(0)
	ModelRun  = M_1DEuler
	CFL       = 0.0
	FinalTime = 100000.
	XMax      = 0.0
	Case      = Euler1D.CaseType(0)
	GridFile  string
	Graph     bool
)

type ModelType uint8

const (
	M_1DAdvect ModelType = iota
	M_1DMaxwell
	M_1DEuler
	M_1DAdvectDFR
	M_1DMaxwellDFR
	M_1DEulerDFR_Roe
	M_1DEulerDFR_LF
	M_1DEulerDFR_Ave
)

var (
	max_CFL  = []float64{1, 1, 3, 3, 1, 2.5, 3, 3}
	def_K    = []int{10, 100, 500, 50, 500, 500, 500, 40}
	def_N    = []int{3, 4, 4, 4, 3, 4, 4, 3}
	def_CFL  = []float64{1, 1, 3, 3, 0.75, 2.5, 3, 0.5}
	def_XMAX = []float64{2 * math.Pi, 1, 1, 2 * math.Pi, 1, 1, 1, 1}
	def_CASE = make([]int, 8)
)

type Model interface {
	Run(graph bool, graphDelay ...time.Duration)
}

func Run() {
	if len(GridFile) != 0 {
		DG2D.ReadGambit2d(GridFile, false)
	}

	var C Model
	switch ModelRun {
	case M_1DAdvect:
		C = Advection1D.NewAdvection(2*math.Pi, CFL, FinalTime, XMax, N, K, Advection1D.GK)
	case M_1DMaxwell:
		C = Maxwell1D.NewMaxwell(CFL, FinalTime, N, K, Maxwell1D.GK)
	case M_1DAdvectDFR:
		C = Advection1D.NewAdvection(2*math.Pi, CFL, FinalTime, XMax, N, K, Advection1D.DFR)
	case M_1DMaxwellDFR:
		C = Maxwell1D.NewMaxwell(CFL, FinalTime, N, K, Maxwell1D.DFR)
	case M_1DEulerDFR_Roe:
		C = Euler1D.NewEuler(CFL, FinalTime, XMax, N, K, Euler1D.DFR_Roe, Case)
	case M_1DEulerDFR_LF:
		C = Euler1D.NewEuler(CFL, FinalTime, XMax, N, K, Euler1D.DFR_LaxFriedrichs, Case)
	case M_1DEulerDFR_Ave:
		C = Euler1D.NewEuler(CFL, FinalTime, XMax, N, K, Euler1D.DFR_Average, Case)
	case M_1DEuler:
		fallthrough
	default:
		C = Euler1D.NewEuler(CFL, FinalTime, XMax, N, K, Euler1D.Galerkin_LF, Case)
	}
	C.Run(Graph, Delay*time.Millisecond)
}

func LimitCFL(model ModelType, CFL float64) (CFLNew float64) {
	var (
		CFLMax float64
	)
	CFLMax = max_CFL[model]
	if CFL > CFLMax {
		fmt.Printf("Input CFL is higher than max CFL for this method\nReplacing with Max CFL: %8.2f\n", CFLMax)
		return CFLMax
	}
	return CFL
}

func Defaults(model ModelType) (CFL, XMax float64, N, K, Case int) {
	return def_CFL[model], def_XMAX[model], def_N[model], def_K[model], def_CASE[model]
}

func getParam(def float64, valP interface{}) float64 {
	switch val := valP.(type) {
	case *int:
		if *val != 0 {
			return float64(*val)
		}
	case *float64:
		if *val != 0 {
			return *val
		}
	}
	return def
}
