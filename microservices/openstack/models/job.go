package models

type JobType int

const (
	CreateVM JobType = iota
	DeleteVM
	AttribVM
	StopVM  // éteindre une VM (off-days) sans la supprimer
	StartVM // rallumer une VM
)

type Job struct {
	Type JobType
	Data map[string]string
}
