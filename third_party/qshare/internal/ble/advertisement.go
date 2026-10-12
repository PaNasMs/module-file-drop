package ble

import (
	"fmt"

	"github.com/ACLzz/qshare/internal/rand"
	"tinygo.org/x/bluetooth"
)

var ble_service_data_base = [14]byte{252, 18, 142, 1, 66, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func NewAdvertisement(
	adapter *bluetooth.Adapter,
	rng rand.Random,
) (*bluetooth.Advertisement, error) {
	ad := adapter.DefaultAdvertisement()
	bleUUID := bluetooth.New16BitUUID(0xfe2c)

	serviceData := make([]byte, len(ble_service_data_base)+10)
	copy(serviceData[0:], ble_service_data_base[:])
	for i := len(ble_service_data_base); i < len(serviceData); i++ {
		serviceData[i] = byte(rng.IntN(256)) // random byte
	}

	err := ad.Configure(bluetooth.AdvertisementOptions{
		ServiceData:       []bluetooth.ServiceDataElement{{UUID: bleUUID, Data: serviceData}},
		AdvertisementType: bluetooth.AdvertisingTypeScanInd,
	})
	if err != nil {
		return nil, fmt.Errorf("configure default ble advertisements: %w", err)
	}

	return ad, nil
}
