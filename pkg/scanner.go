package bacnet

import (
	"fmt"
	"github.com/zyra/bacnet-2/pkg/service"
)

type Res struct {
}

func Scan() *Res {
	var r Res

	devices, err := service.SendWhoIsRequest("docker0")

	if err != nil {
		fmt.Println("Error!", err)
	}

	fmt.Println(len(*devices))

	return &r
}
