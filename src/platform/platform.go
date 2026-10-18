package platform

import "potatonet/src/platform/base"

type Paths = base.Paths

func CurrentExe() string { return base.CurrentExe() }
func PidDir() string     { return base.PidDir() }
