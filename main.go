package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stianeikeland/go-rpio"
	"sethwklein.net/errslice"
	"sethwklein.net/go/webutil"
)

var gpio2physical = []rpio.Pin{
	0,  // no physical or gpio pin 0
	0,  // no gpio pin 1
	3,  // 2
	5,  // 3
	7,  // 4
	29, // 5
	31, // 6
	26, // 7
	24, // 8
	21, // 9
	19, // 10
	23, // 11
	32, // 12
	33, // 13
	8,  // 14
	10, // 15
	12, // 16
	11, // 17
	12, // 18
	35, // 19
	38, // 20
	40, // 21
	15, // 22
	16, // 23
	18, // 24
	22, // 25
	37, // 26
	13, // 27
}

// need to extend this to handle the error case

/*
	type BusTime struct {
		Bustime_response struct {
			Vehicle []struct {
				Blk          int    `json:"blk"`
				Des          string `json:"des"`
				Dly          bool   `json:"dly"`
				Hdg          string `json:"hdg"`
				Lat          string `json:"lat"`
				Lon          string `json:"lon"`
				Oid          string `json:"oid"`
				Or           bool   `json:"or"`
				Pdist        int    `json:"pdist"`
				Pid          int    `json:"pid"`
				Rid          string `json:"rid"`
				Rtpidatafeed string `json:"rtpidatafeed"`
				Spd          int    `json:"spd"`
				Srvtmstmp    string `json:"srvtmstmp"`
				Tablockid    string `json:"tablockid"`
				Tatripid     string `json:"tatripid"`
				Tmstmp       string `json:"tmstmp"`
				Tripid       int    `json:"tripid"`
				Vid          string `json:"vid"`
				Zone         string `json:"zone"`
			} `json:"vehicle"`
		} `json:"bustime-response"`
	}
*/

type BusAPIResponse struct {
	Message struct {
		Error []struct {
			Msg string `json:"msg"`
		} `json:"error"`
		Vehicles []struct {
			Rt  string `json:"rt"`
			Lat string `json:"lat"`
			Lon string `json:"lon"`
		} `json:"vehicle"`
	} `json:"bustime-response"`
}

type BusLoc struct {
	Route string
	Lat   float64
	Lon   float64
}

type LEDs struct {
	Routes map[string]struct {
		Leds []struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
			Pin int     `json:"pin"`
		} `json:"leds"`
	} `json:"routes"`
}

const firstPin = 13
const lastPin = 22

func validGPIOPin(pin int) bool {
	return pin > 0 && pin <= 27
}

func mainError() (err error) {
	var b []byte

	// load led data

	b, err = ioutil.ReadFile("leds.json")
	if err != nil {
		return err
	}

	var leds LEDs
	err = json.Unmarshal(b, &leds)
	if err != nil {
		return err
	}
	fmt.Println(leds)

	// initialize led's

	err = rpio.Open()
	if err != nil {
		return err
	}
	defer errslice.AppendCall(&err, rpio.Close)

	// set all used pins to output and low
	for _, route := range leds.Routes {
		for _, led := range route.Leds {
			if !validGPIOPin(led.Pin) {
				continue
			}
			pin := gpio2physical[led.Pin]
			pin.Output()
			pin.Low()
		}
	}

	// get bus data

	b, err = webutil.GetBytes("http://66.63.112.139/bustime/api/v3/getvehicles?key=692UeGryGbNiuTRqHUxibBCaW&rtpidatafeed=Metro&format=json")
	if err != nil {
		return err
	}

	var r BusAPIResponse
	err = json.Unmarshal(b, &r)
	if err != nil {
		return err
	}

	var buses []BusLoc
	// the bus data uses strings for the location. turn them into floats.
	for _, v := range r.Message.Vehicles {
		var bus BusLoc

		bus.Route = v.Rt

		lat, err := strconv.ParseFloat(v.Lat, 64)
		if err != nil {
			return err
		}
		bus.Lat = lat

		lon, err := strconv.ParseFloat(v.Lon, 64)
		if err != nil {
			return err
		}
		bus.Lon = lon

		buses = append(buses, bus)
	}
	fmt.Println(buses)

	// for each bus on a route we have data for,
	// light the led nearest the bus

	// turn all led's off
	for _, route := range leds.Routes {
		for _, led := range route.Leds {
			if !validGPIOPin(led.Pin) {
				continue
			}
			pin := gpio2physical[led.Pin]
			pin.Low()
		}
	}

	shortestDistance := math.MaxFloat64
	var bestPin int
	for _, bus := range buses {
		route, found := leds.Routes[bus.Route]
		if !found {
			// silent error okay?
			continue
		}
		if len(route.Leds) == 0 {
			continue
		}
		for _, led := range route.Leds {
			latDistance := bus.Lat - led.Lat
			if latDistance < 0 {
				latDistance = -latDistance
			}
			latDistance *= latDistance

			lonDistance := bus.Lon - led.Lon
			if lonDistance < 0 {
				lonDistance = -lonDistance
			}
			lonDistance *= lonDistance

			distance := math.Sqrt(latDistance + lonDistance)

			if distance < shortestDistance {
				shortestDistance = distance
				bestPin = led.Pin
			}
		}

		if !validGPIOPin(bestPin) {
			continue
		}
		fmt.Printf("lighting pin %v\n", bestPin)
		rpio.Pin(bestPin).High()
	}

	time.Sleep(10 * time.Second)

	return nil
}

func mainCode() int {
	err := mainError()
	if err == nil {
		return 0
	}
	fmt.Fprintf(os.Stderr, "%v: Error: %v\n", filepath.Base(os.Args[0]), err)
	return 1
}

func main() {
	os.Exit(mainCode())
}
