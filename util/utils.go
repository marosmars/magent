package util

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
)

func StringOf(value interface{}) string {
	return fmt.Sprintf("[%T%+v]", value, value)
}

func JsonOf(stat interface{}) []byte {
	if value, err := json.Marshal(stat); err != nil {
		log.WithFields(log.Fields{
			"value": stat,
			"err":   err,
		}).Panic("Unable to serialize to JSON")
		panic("Unable to serialize to JSON")
	} else {
		return value
	}
}

func StringOfNoType(value interface{}) string {
	return fmt.Sprintf("%+v", value)
}
