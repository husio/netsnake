package main

import "fmt"

type vec2 struct {
	X int16
	Y int16
}

func (v vec2) Add(other vec2) {
	v.X += other.X
	v.Y += other.Y
}

func (v vec2) Sum(other vec2) vec2 {
	return vec2{
		X: v.X + other.X,
		Y: v.Y + other.Y,
	}
}

func (v vec2) Eq(other vec2) bool {
	return v.X == other.X && v.Y == other.Y
}

func (v vec2) String() string {
	return fmt.Sprintf("vec2{%d, %d}", v.X, v.Y)
}
