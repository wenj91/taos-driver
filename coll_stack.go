package taos


import (
	l "container/list"
)

type stack struct {
	list *l.List
}

func NewStack() *stack {
	list := l.New()
	return &stack{list: list,}
}

func (s *stack) Push(t interface{}){
	s.list.PushFront(t)
}

func  (s *stack) Pop() interface{} {
	ele := s.list.Front()
	if nil != ele {
		s.list.Remove(ele)
		return ele.Value
	}

	return nil
}

func (s *stack) Peak() interface{} {
	ele := s.list.Front()
	return ele.Value
}

func (s *stack) Len() int {
	return s.list.Len()
}

func (s *stack) IsEmpty() bool {
	return s.list.Len() == 0
}