package main

type EntityData struct {
	EntityIndex uint32
	TeamValue   int32
	HealthValue int32
	Name        string
	X, Y, Z     float32
	BonePtr     uint64
	Spotted     bool
}

func ReadControllerLive(reader *ProcessMemoryReader, schema *SchemaManager, ctrlBase uint64) (uint32, string) {
	offPawn := schema.GetFirstFieldOffset("client.dll", [][2]string{{"CCSPlayerController", "m_hPlayerPawn"}})

	offName := uint64(0)
	for _, cand := range [][2]string{{"CCSPlayerController", "m_sSanitizedPlayerName"}, {"CBasePlayerController", "m_sSanitizedPlayerName"}} {
		o, err := schema.GetFieldOffset("client.dll", cand[0], cand[1])
		if err == nil {
			offName = o
			break
		}
	}
	if offName == 0 {
		offName = schema.GetFirstFieldOffset("client.dll", [][2]string{{"CBasePlayerController", "m_iszPlayerName"}, {"CCSPlayerController", "m_iszPlayerName"}})
	}

	pawnHandle := reader.ReadInt32(ctrlBase + offPawn)

	nameStr := ""
	namePtr := reader.ReadUint64(ctrlBase + offName)
	if isProbableUserPointer(namePtr) {
		nameStr = reader.ReadCString(namePtr, 64)
	} else {
		nameStr = reader.ReadCString(ctrlBase+offName, 64)
	}

	if len(nameStr) > 0 {
		isAlnum := false
		for _, c := range nameStr {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
				isAlnum = true
				break
			}
		}
		if !isAlnum {
			nameStr = ""
		}
	}

	return uint32(pawnHandle), nameStr
}

func ReadPawnLive(reader *ProcessMemoryReader, schema *SchemaManager, pawnBase uint64) (int32, int32, float32, float32, float32, uint64) {
	offHealth := schema.GetFirstFieldOffset("client.dll", [][2]string{{"C_CSPlayerPawn", "m_iHealth"}, {"C_BaseEntity", "m_iHealth"}})
	offTeam := schema.GetFirstFieldOffset("client.dll", [][2]string{{"C_CSPlayerPawn", "m_iTeamNum"}, {"C_BaseEntity", "m_iTeamNum"}})
	offScene := schema.GetFirstFieldOffset("client.dll", [][2]string{{"C_BaseEntity", "m_pGameSceneNode"}})
	offModel := schema.GetFirstFieldOffset("client.dll", [][2]string{{"CSkeletonInstance", "m_modelState"}})
	offOrig := schema.GetFirstFieldOffset("client.dll", [][2]string{{"CGameSceneNode", "m_vecAbsOrigin"}})

	health := reader.ReadInt32(pawnBase + offHealth)
	team := reader.ReadInt32(pawnBase + offTeam)

	var x, y, z float32
	var bonePtr uint64
	scenePtr := reader.ReadUint64(pawnBase + offScene)
	if isProbableUserPointer(scenePtr) {
		x = reader.ReadFloat32(scenePtr + offOrig)
		y = reader.ReadFloat32(scenePtr + offOrig + 4)
		z = reader.ReadFloat32(scenePtr + offOrig + 8)
		bonePtr = reader.ReadUint64(scenePtr + offModel + BoneArray)
	}

	return health, team, x, y, z, bonePtr
}

func resolveEntityFromIndex(reader *ProcessMemoryReader, listRoot uint64, idx uint32) uint64 {
	entry := reader.ReadUint64(listRoot + 0x8*(uint64(idx)>>9) + 0x10)
	if !isProbableUserPointer(entry) {
		return 0
	}
	return reader.ReadUint64(entry + 0x70*(uint64(idx)&0x1FF))
}

func resolveEntityFromHandle(reader *ProcessMemoryReader, listRoot uint64, handle uint32) uint64 {
	if handle == 0 || handle == 0xFFFFFFFF {
		return 0
	}
	idx := handle & 0x7FFF
	return resolveEntityFromIndex(reader, listRoot, idx)
}

func enumerateEnemies(reader *ProcessMemoryReader, schema *SchemaManager, maxEnts uint32) ([]EntityData, int32) {
	listRoot := reader.ReadUint64(ClientBase + DwEntityList)
	if !isProbableUserPointer(listRoot) {
		return nil, 0
	}

	var localTeam int32 = 0
	localCtrl := reader.ReadUint64(ClientBase + DwLocalPlayerController)
	if isProbableUserPointer(localCtrl) {
		pHandle, _ := ReadControllerLive(reader, schema, localCtrl)
		pPawn := resolveEntityFromHandle(reader, listRoot, pHandle)
		if isProbableUserPointer(pPawn) {
			_, localTeam, _, _, _, _ = ReadPawnLive(reader, schema, pPawn)
		}
	}

	enemies := make([]EntityData, 0)
	for i := uint32(1); i < maxEnts; i++ {
		ctrlPtr := resolveEntityFromIndex(reader, listRoot, i)
		if !isProbableUserPointer(ctrlPtr) || ctrlPtr == localCtrl {
			continue
		}

		pHandle, name := ReadControllerLive(reader, schema, ctrlPtr)
		if pHandle <= 0 || pHandle == 0xFFFFFFFF {
			continue
		}

		pawnPtr := resolveEntityFromHandle(reader, listRoot, pHandle)
		if !isProbableUserPointer(pawnPtr) {
			continue
		}

		h, t, x, y, z, bp := ReadPawnLive(reader, schema, pawnPtr)
if (t != 2 && t != 3) || h <= 0 || h > 200 {
continue
}

spotted := false
offSpottedState, err := schema.GetFieldOffset("client.dll", "C_CSPlayerPawn", "m_entitySpottedState")
if err == nil && offSpottedState != 0 {
spottedMask := reader.ReadUint64(pawnPtr + offSpottedState + 12)
spotted = (spottedMask != 0) 

bSpotted := reader.ReadBytes(pawnPtr + offSpottedState + 8, 1)
if len(bSpotted) > 0 && bSpotted[0] != 0 {
spotted = true
}
}

		enemies = append(enemies, EntityData{
			EntityIndex: i,
			TeamValue:   t,
			HealthValue: h,
			Name:        name,
			X:           x, Y: y, Z: z,
			BonePtr: bp,
			Spotted: spotted,
		})
	}

	return enemies, localTeam
}
