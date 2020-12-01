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
	"sort"
	"time"

	"github.com/ghodss/yaml"

	"github.com/notargets/avs/chart2d"

	"github.com/notargets/gocfd/model_problems/Euler2D"

	"github.com/spf13/cobra"
)

type Model2D struct {
	K, N           int // Number of elements, Polynomial Degree
	Delay          time.Duration
	PlotSteps      int
	FluxType       Euler2D.FluxType
	CaseType       Euler2D.CaseType
	CFL, FinalTime float64
	GridFile       string
	Graph          bool
	GraphField     int
	ICFile         string
}

var (
	CFL       = 1.
	N         = 1
	FinalTime = 4.
)

type InputParameters struct {
	Title           string                                `yaml:"Title"`
	CFL             float64                               `yaml:"CFL"`
	FluxType        string                                `yaml:"FluxType"`
	Case            string                                `yaml:"Case"`
	PolynomialOrder int                                   `yaml:"PolynomialOrder"`
	BCs             map[string]map[int]map[string]float64 `yaml:"BCs"` // First key is BC name/type, second is parameter name
}

func (ip *InputParameters) Parse(data []byte) error {
	return yaml.Unmarshal(data, ip)
}

func (ip *InputParameters) Print() {
	fmt.Printf("\"%s\"\t\t= Title\n", ip.Title)
	fmt.Printf("%8.5f\t\t= CFL\n", ip.CFL)
	fmt.Printf("[%s]\t\t\t= Flux Type\n", ip.FluxType)
	fmt.Printf("[%s]\t\t= Case\n", ip.Case)
	fmt.Printf("[%d]\t\t\t\t= Polynomial Order\n", ip.PolynomialOrder)
	keys := make([]string, len(ip.BCs))
	i := 0
	for k := range ip.BCs {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("BCs[%s] = %v\n", key, ip.BCs[key])
	}
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
		ct, _ := cmd.Flags().GetInt("caseType")
		m2d.CaseType = Euler2D.CaseType(ct)
		ft, _ := cmd.Flags().GetInt("fluxType")
		m2d.FluxType = Euler2D.FluxType(ft)
		dr, _ := cmd.Flags().GetInt("delay")
		m2d.Delay = time.Duration(time.Duration(dr) * time.Millisecond)
		ps, _ := cmd.Flags().GetInt("plotSteps")
		m2d.PlotSteps = ps
		m2d.FinalTime, _ = cmd.Flags().GetFloat64("finalTime")
		m2d.CFL, _ = cmd.Flags().GetFloat64("CFL")
		if m2d.GridFile, err = cmd.Flags().GetString("gridFile"); err != nil {
			panic(err)
		}
		if m2d.ICFile, err = cmd.Flags().GetString("inputConditionsFile"); err != nil {
			panic(err)
		}
		m2d.Graph, _ = cmd.Flags().GetBool("graph")
		m2d.GraphField, _ = cmd.Flags().GetInt("graphField")
		m2d.N, _ = cmd.Flags().GetInt("n")
		processInput(m2d)
		Run2D(m2d)
	},
}

func processInput(m2d *Model2D) {
	var (
		err error
	)
	if len(m2d.ICFile) != 0 {
		var data []byte
		if data, err = ioutil.ReadFile(m2d.ICFile); err != nil {
			panic(err)
		}
		var input InputParameters
		if err = input.Parse(data); err != nil {
			panic(err)
		}
		fmt.Println("Input = ", input)
	}
	if len(m2d.GridFile) == 0 {
		err := fmt.Errorf("must have a grid file in .neu (Gambit neutral file) format")
		panic(err)
	}
}

func init() {
	rootCmd.AddCommand(TwoDCmd)
	TwoDCmd.Flags().IntP("caseType", "c", int(1), "type of model, eg: 0 for freestream, 1 for vortex")
	TwoDCmd.Flags().IntP("fluxType", "f", int(1), "type of flux calculation, eg: 1 for Lax, 2 for Roe")
	TwoDCmd.Flags().IntP("n", "n", N, "polynomial degree")
	TwoDCmd.Flags().IntP("delay", "d", 0, "milliseconds of delay for plotting")
	TwoDCmd.Flags().IntP("plotSteps", "s", 1, "number of steps before plotting each frame")
	TwoDCmd.Flags().BoolP("graph", "g", false, "display a graph while computing solution")
	TwoDCmd.Flags().IntP("graphField", "q", 0, "which field should be displayed - 0=density, 1,2=momenta, 3=energy")
	TwoDCmd.Flags().Float64("CFL", CFL, "CFL - increase for speedup, decrease for stability")
	TwoDCmd.Flags().Float64("finalTime", FinalTime, "FinalTime - the target end time for the sim")
	TwoDCmd.Flags().StringP("gridFile", "F", "", "Grid file to read in Gambit (.neu) format")
	TwoDCmd.Flags().StringP("inputConditionsFile", "I", "", "YAML file for input parameters like:\n\t- CFL\n\t- NPR (nozzle pressure ratio)")
}

func Run2D(m2d *Model2D) {
	c := Euler2D.NewEuler(
		m2d.FinalTime, m2d.N, m2d.GridFile, m2d.CFL, m2d.FluxType, m2d.CaseType,
		false, true)
	pm := &Euler2D.PlotMeta{
		Plot:            m2d.Graph,
		Scale:           1.1,
		Field:           Euler2D.PlotField(m2d.GraphField),
		FieldMinP:       nil,
		FieldMaxP:       nil,
		FrameTime:       m2d.Delay,
		StepsBeforePlot: m2d.PlotSteps,
		LineType:        chart2d.NoLine,
	}
	c.Solve(pm)
}
