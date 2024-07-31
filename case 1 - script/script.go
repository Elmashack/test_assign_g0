package main

import (
	context2 "context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"
	"time"
)

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MaxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type WeightedPoint struct {
	Point
	Weight float32 `json:"weight"`
}

type Polygon struct {
	Points []WeightedPoint `json:"points"`
}

type Task struct {
	idx     int
	polygon *Polygon
}

type Bbox struct {
	x1 int
	y1 int
	x2 int
	y2 int
}

type Result struct {
	Bbox          Bbox
	MaxWeight     float32
	HeavyPolygons []*Polygon
}

var timeout = flag.Int("timeout", 60, "maximal processing time in seconds")

var polygonsNum = flag.Int("polygons_num", 3, "number of polygons to process")

func main() {
	flag.Parse()

	ctx, cancel := context2.WithTimeout(context2.Background(), time.Second*time.Duration(*timeout))
	defer cancel()

	var polygons []Polygon
	for i := 0; i < *polygonsNum; i++ {
		resp, err := http.DefaultClient.Get("http://localhost:8080/polygon")
		if err != nil {
			fmt.Printf("fail")
			os.Exit(1)
		}
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("fail")
			os.Exit(1)
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("fail")
			os.Exit(1)
		}
		poly := Polygon{}
		err = json.Unmarshal(respBody, &poly)
		if err != nil {
			fmt.Printf("fail")
		}

		polygons = append(polygons, poly)
	}

	tasks := make(chan Task, *polygonsNum)
	for idx, poly := range polygons {
		tasks <- Task{idx: idx, polygon: &poly}
	}
	close(tasks)

	result := Result{
		Bbox: Bbox{
			x1: math.MaxInt,
			y1: math.MaxInt,
			x2: math.MinInt,
			y2: math.MinInt,
		},
		MaxWeight: 0,
	}
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		go func() {
			wg.Add(1)
			for {
				select {
				case <-ctx.Done():
					wg.Done()
					return
				case task, ok := <-tasks:
					if !ok {
						wg.Done()
						return
					}
					sumWeight := float32(0)
					for _, p := range task.polygon.Points {
						sumWeight += p.Weight
						result.Bbox.x1 = MinInt(result.Bbox.x1, p.X)
						result.Bbox.y1 = MinInt(result.Bbox.y1, p.Y)
						result.Bbox.x2 = MaxInt(result.Bbox.x2, p.X)
						result.Bbox.y2 = MaxInt(result.Bbox.y2, p.Y)
					}
					result.MaxWeight = MaxFloat32(result.MaxWeight, sumWeight)
					if sumWeight > 100 {
						result.HeavyPolygons = append(result.HeavyPolygons, task.polygon)
					}
				}
			}
		}()
	}
	wg.Wait()

	output, err := json.MarshalIndent(result, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(output))
}
