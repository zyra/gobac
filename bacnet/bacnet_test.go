package bacnet

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/suite"
	"github.com/zyra/gobac/bacnet/types"
	"testing"
	"time"
)

var ifname = "docker0"
var err error
var ctx context.Context

type BacnetTestSuite struct {
	suite.Suite
	Config *ServerConfig
	Server Server

	Devices []*types.Device
	Device  *types.Device
	DevCtrl *DeviceController

	Objects []*types.Object
}

func (s *BacnetTestSuite) SetupSuite() {
	s.Config = NewServerConfig()

	s.Config.SetInterfaceName(ifname).SetDefaultTimeout(time.Second * 3)

	s.Server, err = NewServer(s.Config)

	s.NoError(err)

	if err != nil {
		return
	}

	s.Equal(s.Server.InterfaceName, ifname)

	go s.Server.Listen(ctx)

	select {
	case <-s.Server.Start():
	case <-ctx.Done():
	}
}

func (s *BacnetTestSuite) TearDownSuite() {
	s.Server.Close()
}

var count = 0

func (s *BacnetTestSuite) Test1Scan() {
	dChan, err := s.Server.WhoIs(context.TODO(), time.Millisecond*500)

	s.NoError(err)

	if err != nil {
		return
	}

	for {
		dev, open := <-dChan

		if dev != nil {
			s.Devices = append(s.Devices, dev)
		}

		if !open {
			break
		}
		count++
	}

	s.NotEmpty(s.Devices)

	if len(s.Devices) == 0 {
		return
	}

	s.Device = s.Devices[0]

	ctrl := DeviceController(*s.Device)
	s.DevCtrl = &ctrl
}

func (s *BacnetTestSuite) Test2Objects() {
	s.NotEmpty(s.Devices)
	s.NotNil(s.DevCtrl)

	objs, err := s.DevCtrl.GetObjects(s.Server)

	s.NoError(err)

	if err != nil {
		return
	}

	s.Objects = objs

	s.NotEmpty(s.Objects)
}

func (s *BacnetTestSuite) Test3Write() {
	s.NotEmpty(s.Objects)

	var obj *types.Object

	for _, o := range s.Objects {
		if o.ObjectId.Type == types.ObjectTypeAnalogValue {
			obj = o
			break
		}
	}

	s.NotNil(obj)

	if obj == nil {
		return
	}

	objCtrl := ObjectController(*obj)

	prop, err := objCtrl.GetProperty(s.Server, types.PropertyPresentValue)

	s.NoError(err)

	if err != nil {
		return
	}

	propCtrl := PropertyController(*prop)

	currentVal := prop.Values[0].ReadAsFloat64Unsafe()
	newVal := currentVal + 0.5

	if err := propCtrl.SetValue(context.TODO(), s.Server, TagReal, newVal); err != nil {
		s.NoError(err)
	} else {
		prop, err = objCtrl.GetProperty(s.Server, types.PropertyPresentValue)

		if err != nil {
			s.NoError(err)
			return
		}

		updatedVal := prop.Values[0].ReadAsFloat64Unsafe()

		s.NotEqual(currentVal, updatedVal)
		s.Equal(newVal, updatedVal)
	}
}

func (s *BacnetTestSuite) Test4ReadAll() {
	for i, o := range s.Objects {
		if i == 0 {
			// skip device
			continue
		}

		objCtrl := ObjectController(*o)

		if p, err := objCtrl.GetProperty(s.Server, types.PropertyObjectName); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("name is ", p.Values[0].ReadAsString())
		}

		if p, err := objCtrl.GetProperty(s.Server, types.PropertyStateText); err == nil && p != nil {
			fmt.Println(p.Values[0].ReadAsString(), len(p.Values))
		}

		if _, err := objCtrl.GetProperty(s.Server, types.PropertyPresentValue); err != nil {
			fmt.Println(err)
		}

		if p, err := objCtrl.GetProperty(s.Server, types.PropertyDescription); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("> Description is %s\n", p.Values[0].Value)
		}
	}
}

func (s *BacnetTestSuite) Test5Server_SendCovRequest() {
	var obj *types.Object

	for _, o := range s.Objects {
		if ObjectController(*o).ObjectId.Type == types.ObjectTypeAnalogValue {
			obj = o
			break
		}
	}

	if !s.NotNil(obj) {
		return
	}

	objCtrl := ObjectController(*obj)

	prop, err := objCtrl.GetProperty(s.Server, types.PropertyPresentValue)

	if !s.NoError(err) {
		return
	}

	propCtrl := PropertyController(*prop)

	req, err := s.Server.SubscribeCov(context.TODO(), obj.IPAddress, obj.ObjectId.Type, obj.ObjectId.Instance, 5, false)

	if !s.NoError(err) {
		return
	}

	defer func() {
		println("unsubscribing")
		_, err = s.Server.SubscribeCov(context.TODO(), obj.IPAddress, obj.ObjectId.Type, obj.ObjectId.Instance, 5, true)

		if !s.NoError(err) {
			return
		}
	}()

	fmt.Println("sent cov req")

	err = propCtrl.SetValue(context.TODO(), s.Server, TagReal, 2)

	if !s.NoError(err) {
		return
	}

	fmt.Println("Value was set to 2")

	select {
	case <-time.After(time.Second * 3):
		s.Fail("did not receive a COV notification after 3 seconds")

	case n := <-req.Data():
		if !s.Equal(objCtrl.ObjectId.Type, n.ObjectId.Type) || !s.Equal(objCtrl.ObjectId.Instance, n.ObjectId.Instance) {
			return
		}

		var prop *types.PropertyValue

		for _, p := range n.Properties {
			if p.Values != nil && len(p.Values) > 0 && p.Values[0].Type == types.TagReal {
				prop = p.Values[0]
				break
			}
		}

		if !s.NotNil(prop) {
			return
		}

		fmt.Println("Got a property!", prop)
		if !s.NotNil(prop.Value) {
			return
		}

		if !s.EqualValues(2, prop.Value.(types.Real)) {
			return
		}

		fmt.Println("all good!")

	case err = <-req.Error():
		if !s.NoError(err) {
			return
		}
	}
}

func TestBacnet(t *testing.T) {
	suite.Run(t, new(BacnetTestSuite))
}
