package spoe

import (
	"github.com/pkg/errors"
)

type Arg struct {
	Name  string
	Value interface{}
}

type ArgIterator struct {
	b     []byte
	count int
	Arg   Arg
	err   error
}

func (i *ArgIterator) Next() bool {
	if i.count == 0 {
		return false
	}
	name, value, n, err := decodeKV(i.b)
	if err != nil {
		i.err = err
		return false
	}
	i.b = i.b[n:]

	i.Arg.Name = name
	i.Arg.Value = value
	i.count--
	return true
}

func (i *ArgIterator) Map() map[string]interface{} {
	res := make(map[string]interface{}, i.count)
	for i.Next() {
		res[i.Arg.Name] = i.Arg.Value
	}

	return res
}

func (i *ArgIterator) Count() int {
	return i.count
}

type Message struct {
	Name string
	Args *ArgIterator
}

type MessageIterator struct {
	b   []byte
	err error

	Message Message
}

func (i *MessageIterator) Error() error {
	return i.err
}

func (i *MessageIterator) Next() bool {
	if i.Message.Args != nil {
		for i.Message.Args.Next() {
		}

		i.b = i.Message.Args.b

		if i.Message.Args.err != nil {
			i.err = i.Message.Args.err
			return false
		}
	}

	if len(i.b) == 0 {
		return false
	}

	messageName, n, err := decodeString(i.b)
	if err != nil {
		i.err = err
		return false
	}
	i.b = i.b[n:]

	argCount := int(i.b[0])
	i.b = i.b[1:]

	i.Message.Args.b = i.b
	i.Message.Args.count = argCount
	i.Message.Name = messageName

	return true
}

func (c *conn) handleNotify(f Frame, acks chan Frame) error {
	messages := &MessageIterator{
		b: f.data,
		Message: Message{
			Args: &ArgIterator{
				b: f.data,
			},
		},
	}

	actions, err := c.handler(messages)
	if err != nil {
		return errors.Wrap(err, "handle notify") // TODO return proper response
	}

	f.ftype = frameTypeAgentACK
	f.flags = frameFlagFin
	f.data = f.originalData

	off := 0

	for _, a := range actions {
		n, err := a.encode(f.data[off:])
		if err != nil {
			return errors.Wrap(err, "handle notify")
		}
		off += n
	}

	f.data = f.data[:off]

	acks <- f

	return nil
}
