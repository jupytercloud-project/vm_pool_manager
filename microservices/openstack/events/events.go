package events

import "PoolManagerVM/backend/pb"

type RessourceEvent struct {
	Action    string
	Type      pb.Type
	Ressource any
}
