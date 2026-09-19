package model

import "time"

type Window struct{
	Limit int 
	Window time.Duration
}