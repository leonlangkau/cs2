package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

var (
	ClientBase              uint64
	DwViewMatrix            uint64
	DwEntityList            uint64
	DwLocalPlayerController uint64
	DwLocalPlayerPawn       uint64
	DwViewAngles            uint64
	DwForceAttack           uint64
	BoneArray               uint64 = 0x80
)

func loadFromOffsetsJSON(path string, module string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var parsed map[string]map[string]float64
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	mod, ok := parsed[module]
	if !ok {
		return fmt.Errorf("module %s not found in offsets", module)
	}

	buttonsData, err := ioutil.ReadFile(`C:\Users\jjcan\Downloads\cs2-dumper-main\cs2-dumper-main\output\buttons.json`)
	var buttonsParsed map[string]map[string]interface{}
	if err == nil {
		if jsonErr := json.Unmarshal(buttonsData, &buttonsParsed); jsonErr == nil {
			if m, ok := buttonsParsed["client.dll"]; ok {
				if v, ok := m["attack"].(float64); ok {
					DwForceAttack = uint64(v)
				} else if v, ok := m["attack"].(int); ok {
					DwForceAttack = uint64(v)
				}
			}
		}
	}

	DwViewMatrix = uint64(mod["dwViewMatrix"])
	DwEntityList = uint64(mod["dwEntityList"])
	DwLocalPlayerController = uint64(mod["dwLocalPlayerController"])
	DwLocalPlayerPawn = uint64(mod["dwLocalPlayerPawn"])
	DwViewAngles = uint64(mod["dwViewAngles"])
	return nil
}

type SchemaManager struct {
	Schema map[string]interface{}
}

func NewSchemaManager(path string) (*SchemaManager, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return &SchemaManager{Schema: schema}, nil
}

func (sm *SchemaManager) GetFieldOffset(module, className, fieldName string) (uint64, error) {
	m, ok := sm.Schema[module].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("module %s not found", module)
	}
	classes, ok := m["classes"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("classes missing")
	}
	cls, ok := classes[className].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("class %s not found", className)
	}
	fields, ok := cls["fields"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("fields missing")
	}
	field, ok := fields[fieldName].(float64)
	if !ok {
		return 0, fmt.Errorf("field %s not found", fieldName)
	}
	return uint64(field), nil
}

func (sm *SchemaManager) GetFirstFieldOffset(module string, candidates [][2]string) uint64 {
	for _, cand := range candidates {
		off, err := sm.GetFieldOffset(module, cand[0], cand[1])
		if err == nil {
			return off
		}
	}
	panic("Could not find any matching offset candidates")
}
