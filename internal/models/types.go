package models

type Mappable interface {
	Table() Table
	Map() map[string]any
}

type Postable interface {
	Mappable
	String() (string, error)
	GetGuildID() string
	GetChannelID() string
	GetMessageID() string
	SetGuildID(string)
	SetChannelID(string)
	SetMessageID(string)
}
