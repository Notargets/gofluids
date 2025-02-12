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
	"io/ioutil"
	"os"
	"time"

	"github.com/notargets/gocfd/InputParameters"

	"github.com/notargets/avs/chart2d"

	"github.com/notargets/gocfd/model_problems/Euler2D"

	"github.com/spf13/cobra"
)

type Model2D struct {
	GridFile                     string
	PlotMesh                     bool
	NOrder                       int
	ICFile                       string
	Graph                        bool
	GraphField                   int
	PlotSteps                    int
	Delay                        time.Duration
	ParallelProcLimit            int
	Profile                      bool
	Zoom, TranslateX, TranslateY float64
	fminP, fmaxP                 *float64
}

// TwoDCmd represents the 2D command
var TwoDCmd = &cobra.Command{
	Use:   "2D",
	Short: "Two dimensional solver, able to read grid files and output solutions",
	Long:  `Two dimensional solver, able to read grid files and output solutions`,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			err error
		)
		fmt.Println("2D called")
		m2d := &Model2D{}
		if m2d.GridFile, err = cmd.Flags().GetString("gridFile"); err != nil {
			panic(err)
		}
		m2d.PlotMesh, _ = cmd.Flags().GetBool("plotMesh")
		m2d.NOrder, _ = cmd.Flags().GetInt("nOrder")
		if m2d.ICFile, err = cmd.Flags().GetString("inputConditionsFile"); err != nil {
			panic(err)
		}
		m2d.Graph, _ = cmd.Flags().GetBool("graph")
		m2d.GraphField, _ = cmd.Flags().GetInt("graphField")
		ps, _ := cmd.Flags().GetInt("plotSteps")
		m2d.PlotSteps = ps
		dr, _ := cmd.Flags().GetInt("delay")
		m2d.Delay = time.Duration(dr) * time.Millisecond
		m2d.ParallelProcLimit, _ = cmd.Flags().GetInt("parallelProcs")
		m2d.Zoom, _ = cmd.Flags().GetFloat64("zoom")
		m2d.TranslateX, _ = cmd.Flags().GetFloat64("translateX")
		m2d.TranslateY, _ = cmd.Flags().GetFloat64("translateY")
		fmin, _ := cmd.Flags().GetFloat64("plotMin")
		fmax, _ := cmd.Flags().GetFloat64("plotMax")
		m2d.Profile, _ = cmd.Flags().GetBool("profile")
		if fmin != -1000 {
			m2d.fminP = &fmin
		}
		if fmax != -1000 {
			m2d.fmaxP = &fmax
		}
		pm := &InputParameters.PlotMeta{
			Plot:            m2d.Graph,
			PlotMesh:        m2d.PlotMesh,
			Field:           uint16(m2d.GraphField),
			FieldMinP:       m2d.fminP,
			FieldMaxP:       m2d.fmaxP,
			FrameTime:       m2d.Delay,
			StepsBeforePlot: m2d.PlotSteps,
			LineType:        chart2d.NoLine,
			Scale:           m2d.Zoom,
			TranslateX:      m2d.TranslateX,
			TranslateY:      m2d.TranslateY,
		}

		ip := processInput(m2d, pm.PlotMesh)
		Run2D(m2d, ip, pm)
	},
}

func processInput(m2d *Model2D, plotMesh bool) (ip *InputParameters.InputParameters2D) {
	var (
		err      error
		willExit = false
	)
	if len(m2d.GridFile) == 0 {
		err := fmt.Errorf("must supply a grid file (-F, --gridFile) in .neu (Gambit neutral file) format")
		fmt.Printf("error: %s\n", err.Error())
		willExit = true
	}
	if plotMesh && m2d.NOrder == -1 {
		err = fmt.Errorf("must supply an order (-nOrder) for the mesh")
		fmt.Println(err)
		fmt.Println("exiting")
		os.Exit(1)
	}
	if len(m2d.ICFile) == 0 {
		err := fmt.Errorf("must supply an input parameters file (-I, --inputConditionsFile) in .neu (Gambit neutral file) format")
		fmt.Printf("error: %s\n", err.Error())
		exampleFile := `
########################################
Title: "Test Case"
CFL: 1.
FluxType: Lax
InitType: IVortex # Can be "Freestream"
PolynomialOrder: 1
FinalTime: 4
########################################
`
		fmt.Printf("Example File Contents:%s\n", exampleFile)
		if !plotMesh {
			willExit = true
		}
	}
	if willExit {
		fmt.Println("exiting")
		os.Exit(1)
	}
	ip = &InputParameters.InputParameters2D{}
	ip.Gamma = 1.4 // Default
	ip.Minf = 0.1  // Default
	var data []byte
	if len(m2d.ICFile) != 0 {
		if data, err = ioutil.ReadFile(m2d.ICFile); err != nil {
			panic(err)
		}
		if err = ip.Parse(data); err != nil {
			panic(err)
		}
	} else {
		ip.PolynomialOrder = m2d.NOrder // Default
		ip.FluxType = "Lax"             // Default
		ip.InitType = "Freestream"
	}
	return
}

func init() {
	rootCmd.AddCommand(TwoDCmd)
	TwoDCmd.Flags().StringP("gridFile", "F", "", "Grid file to read in Gambit (.neu) or SU2 (.su2) format")
	TwoDCmd.Flags().BoolP("plotMesh", "m", false, "plot the input mesh and exit")
	TwoDCmd.Flags().IntP("nOrder", "N", -1, "order of the polynomial used for the mesh")
	TwoDCmd.Flags().StringP("inputConditionsFile", "I", "", "YAML file for input parameters like:\n\t- CFL\n\t- NPR (nozzle pressure ratio)")
	TwoDCmd.Flags().BoolP("graph", "g", false, "display a graph while computing solution")
	TwoDCmd.Flags().IntP("delay", "d", 0, "milliseconds of delay for plotting")
	TwoDCmd.Flags().IntP("plotSteps", "s", 1, "number of steps before plotting each frame")
	TwoDCmd.Flags().IntP("graphField", "q", 0, "which field should be displayed - 0=density, 1,2=momenta, 3=energy")
	TwoDCmd.Flags().Float64P("zoom", "z", 1.1, "zoom level for plotting")
	TwoDCmd.Flags().Float64P("translateX", "x", 0, "translation in X for plotting")
	TwoDCmd.Flags().Float64P("translateY", "y", 0, "translation in X for plotting")
	TwoDCmd.Flags().IntP("parallelProcs", "p", 0, "limits the parallelism to the number of specified processes")
	TwoDCmd.Flags().Float64P("plotMin", "k", -1000, "field min for plotting")
	TwoDCmd.Flags().Float64P("plotMax", "l", -1000, "field max for plotting")
	TwoDCmd.Flags().Bool("profile", false, "generate a runtime profile of the solver, can be converted to PDF using 'go tool pprof -pdf filename'")
}

func Run2D(m2d *Model2D, ip *InputParameters.InputParameters2D, pm *InputParameters.PlotMeta) {
	// fmt.Printf("m2d: %+v\n", m2d)
	// fmt.Printf("ip: %+v\n", ip)
	// fmt.Printf("pm: %+v\n", pm)
	// os.Exit(1)
	c := Euler2D.NewEuler(ip, pm, m2d.GridFile, m2d.ParallelProcLimit, true, m2d.Profile)
	c.SaveOutputMesh("meshfile.gcfd")
	/*
		if ip.ImplicitSolver {
			c.SolveImplicit(pm)
		} else {
			c.Solve(pm)
		}
	*/
	c.Solve(pm)
}
