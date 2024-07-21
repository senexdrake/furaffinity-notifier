package fa

import "sync"

var mapMutex sync.RWMutex
var userMap = make(map[string]*FurAffinityUser)
