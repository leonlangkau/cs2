package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/inkyblackness/imgui-go/v4"
)

var (
	moduser32   = syscall.NewLazyDLL("user32.dll")
	modgdi32    = syscall.NewLazyDLL("gdi32.dll")
	modopengl32 = syscall.NewLazyDLL("opengl32.dll")

	procGetDC            = moduser32.NewProc("GetDC")
	procReleaseDC        = moduser32.NewProc("ReleaseDC")
	procSetWindowLongW   = moduser32.NewProc("SetWindowLongW")
	procGetWindowLongW   = moduser32.NewProc("GetWindowLongW")
	procSetWindowPos     = moduser32.NewProc("SetWindowPos")
	procGetAsyncKeyState = moduser32.NewProc("GetAsyncKeyState")
	procGetCursorPos     = moduser32.NewProc("GetCursorPos")

	procCreateFontW        = modgdi32.NewProc("CreateFontW")
	procSelectObject       = modgdi32.NewProc("SelectObject")
	procWglUseFontBitmapsW = modopengl32.NewProc("wglUseFontBitmapsW")
	procMouseEvent         = moduser32.NewProc("mouse_event")
)

const (
	GWL_EXSTYLE       = -20
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TRANSPARENT = 0x00000020
	HWND_TOPMOST      = ^uintptr(0) // -1
	SWP_NOMOVE        = 0x0002
	SWP_NOSIZE        = 0x0001

	VK_INSERT  = 0x2D
	VK_LBUTTON = 0x01

	MOUSEEVENTF_MOVE = 0x0001
)

type POINT struct {
	X, Y int32
}

func isKeyPressed(vk int) bool {
	res, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (res & 0x8000) != 0
}

func getCursorPos() (float32, float32) {
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return float32(pt.X), float32(pt.Y)
}

func moveMouse(dx, dy int32) {
	procMouseEvent.Call(uintptr(MOUSEEVENTF_MOVE), uintptr(dx), uintptr(dy), 0, 0)
}

func drawGLText(x, y float32, text string, r, g, b float32, screenW, screenH float32) {
	if text == "" {
		return
	}

	ndcX := (x/screenW)*2 - 1
	// Offset text slightly so it aligns nicely
	ndcY := 1 - ((y+10)/screenH)*2

	gl.Color3f(r, g, b)
	gl.RasterPos2f(ndcX, ndcY)

	gl.PushAttrib(gl.LIST_BIT)
	gl.ListBase(1000)

	buf := []byte(text)
	gl.CallLists(int32(len(buf)), gl.UNSIGNED_BYTE, unsafe.Pointer(&buf[0]))
	gl.PopAttrib()
}

// -------------------------------------------------------------
// ImGui Binding Structures
// -------------------------------------------------------------

func buildImGuiFontTexture(ctx *imgui.Context) uint32 {
	io := imgui.CurrentIO()
	image := io.Fonts().TextureDataAlpha8()

	var textureId uint32
	gl.GenTextures(1, &textureId)
	gl.BindTexture(gl.TEXTURE_2D, textureId)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.ALPHA, int32(image.Width), int32(image.Height), 0, gl.ALPHA, gl.UNSIGNED_BYTE, image.Pixels)

	io.Fonts().SetTextureID(imgui.TextureID(textureId))

	return textureId
}

func renderImGuiDrawData(drawData imgui.DrawData, width float32, height float32) {
	gl.PushAttrib(gl.ENABLE_BIT | gl.COLOR_BUFFER_BIT | gl.TRANSFORM_BIT)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.CULL_FACE)
	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.SCISSOR_TEST)
	gl.Enable(gl.TEXTURE_2D)
	gl.EnableClientState(gl.VERTEX_ARRAY)
	gl.EnableClientState(gl.TEXTURE_COORD_ARRAY)
	gl.EnableClientState(gl.COLOR_ARRAY)

	gl.MatrixMode(gl.PROJECTION)
	gl.PushMatrix()
	gl.LoadIdentity()
	gl.Ortho(0, float64(width), float64(height), 0, -1, 1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()

	var vertexSize = 20
	var vertexPointerOffset = 0
	var uvPointerOffset = 8
	var colorPointerOffset = 16

	for _, list := range drawData.CommandLists() {
		vertexBuffer, _ := list.VertexBuffer()
		indexBuffer, _ := list.IndexBuffer()
		idxBufferOffset := 0

		gl.VertexPointer(2, gl.FLOAT, int32(vertexSize), unsafe.Pointer(uintptr(vertexBuffer)+uintptr(vertexPointerOffset)))
		gl.TexCoordPointer(2, gl.FLOAT, int32(vertexSize), unsafe.Pointer(uintptr(vertexBuffer)+uintptr(uvPointerOffset)))
		gl.ColorPointer(4, gl.UNSIGNED_BYTE, int32(vertexSize), unsafe.Pointer(uintptr(vertexBuffer)+uintptr(colorPointerOffset)))

		for _, cmd := range list.Commands() {
			if cmd.HasUserCallback() {
				cmd.CallUserCallback(list)
			} else {
				gl.BindTexture(gl.TEXTURE_2D, uint32(cmd.TextureID()))

				clipRect := cmd.ClipRect()
				gl.Scissor(int32(clipRect.X), int32(height-clipRect.W), int32(clipRect.Z-clipRect.X), int32(clipRect.W-clipRect.Y))

				gl.DrawElements(gl.TRIANGLES, int32(cmd.ElementCount()), gl.UNSIGNED_SHORT, unsafe.Pointer(uintptr(indexBuffer)+uintptr(idxBufferOffset)))
			}
			idxBufferOffset += cmd.ElementCount() * 2
		}
	}

	gl.MatrixMode(gl.MODELVIEW)
	gl.PopMatrix()
	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.PopAttrib()
}

func init() {
	runtime.LockOSThread()
}

func main() {
	offsetsPath := flag.String("offsets", `C:\Users\jjcan\Downloads\cs2-dumper-main\cs2-dumper-main\output\offsets.json`, "Offsets JSON path")
	schemaPath := flag.String("schema", `C:\Users\jjcan\Downloads\cs2-dumper-main\cs2-dumper-main\output\client_dll.json`, "Schema JSON path")
	procName := flag.String("proc", "cs2.exe", "Process name")
	flag.Parse()

	if err := loadFromOffsetsJSON(*offsetsPath, "client.dll"); err != nil {
		fmt.Printf("Failed to load offsets: %v\n", err)
		return
	}
	schema, err := NewSchemaManager(*schemaPath)
	if err != nil {
		fmt.Printf("Failed to load schema: %v\n", err)
		return
	}

	reader, err := NewProcessMemoryReader(*procName)
	if err != nil {
		fmt.Printf("Failed to attach to process: %v\n", err)
		return
	}
	defer reader.Close()

	cBase, err := reader.GetModuleBase("client.dll")
	if err != nil {
		fmt.Printf("Failed to get client.dll base: %v\n", err)
		return
	}
	ClientBase = cBase

	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Floating, glfw.True)
	glfw.WindowHint(glfw.Decorated, glfw.False)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.TransparentFramebuffer, glfw.True)
	glfw.WindowHint(glfw.Samples, 4)

	monitor := glfw.GetPrimaryMonitor()
	mode := monitor.GetVideoMode()
	width := mode.Width
	height := mode.Height

	window, err := glfw.CreateWindow(width, height, "CS2 Overlay", nil, nil)
	if err != nil {
		panic(err)
	}

	hwnd := window.GetWin32Window()
	hwndPtr := uintptr(unsafe.Pointer(hwnd))

	gwlExStyle := uintptr(^uint32(0)) - 19
	exStyle, _, _ := procGetWindowLongW.Call(hwndPtr, gwlExStyle)
	// Make it transparent but NOT click-through initially so we can click ImGui!
	procSetWindowLongW.Call(hwndPtr, gwlExStyle, exStyle|WS_EX_LAYERED)
	procSetWindowPos.Call(hwndPtr, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE)

	window.MakeContextCurrent()
	glfw.SwapInterval(1)

	if err := gl.Init(); err != nil {
		panic(err)
	}

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// SETUP NATIVE OPENGL TEXT (For ESP overlay when menu is closed)
	dc, _, _ := procGetDC.Call(hwndPtr)
	fontName, _ := syscall.UTF16PtrFromString("Consolas")
	hFont, _, _ := procCreateFontW.Call(
		uintptr(18), 0, 0, 0, uintptr(400),
		0, 0, 0, 0,
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(fontName)),
	)
	procSelectObject.Call(dc, hFont)
	procWglUseFontBitmapsW.Call(dc, 0, 255, 1000)
	procReleaseDC.Call(hwndPtr, dc)

	// ImGui Context Setup
	imguiCtx := imgui.CreateContext(nil)
	defer imguiCtx.Destroy()

	io := imgui.CurrentIO()
	io.SetDisplaySize(imgui.Vec2{X: float32(width), Y: float32(height)})
	buildImGuiFontTexture(imguiCtx)

	fmt.Println("Overlay running in Go. Press INSERT to toggle Menu Click-Through.")

	boneConnections := [][2]uint64{
		{6, 5}, {5, 4}, {4, 0},
		{5, 8}, {8, 9}, {9, 10},
		{5, 13}, {13, 14}, {14, 15},
		{0, 22}, {22, 23}, {23, 24},
		{0, 25}, {25, 26}, {26, 27},
	}

	var lastUpdate time.Time
	var matrix []float32
	var enemies []EntityData
	var localTeam int32

	fW := float32(width)
	fH := float32(height)

	showMenu := true
	espEnabled := true
	showTeam := false
	showBones := true
	showNames := true
	legitbotEnabled := false
	legitbotSmooth := float32(1.0)
	legitbotFov := float32(100.0)

	ragebotEnabled := false
	ragebotFov := float32(100.0)
	psilentEnabled := false
	visCheckEnabled := false
noVisualRecoil := false
rcsEnabled := false

	drawFov := false

	var lastInsertPress time.Time
	var lastShotTime time.Time
	
	
	lastTime := time.Now()

	for !window.ShouldClose() {
		// Delta Time
		now := time.Now()
		deltaTime := float32(now.Sub(lastTime).Seconds())
if deltaTime <= 0.0001 {
deltaTime = 0.0001
}
lastTime = now
_ = lastShotTime
io.SetDeltaTime(deltaTime)

		// Poll Windows mouse directly and feed it to ImGui
		mouseX, mouseY := getCursorPos()
		io.SetMousePosition(imgui.Vec2{X: mouseX, Y: mouseY})
		io.SetMouseButtonDown(0, isKeyPressed(VK_LBUTTON))

		gl.ClearColor(0, 0, 0, 0)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		glfw.PollEvents()

		// Key checks with cooldown for native menu
		if isKeyPressed(VK_INSERT) && now.Sub(lastInsertPress).Seconds() > 0.2 {
			showMenu = !showMenu
			lastInsertPress = now

			// Toggle Window Click-Through
			if showMenu {
				// Menu Open = App catches mouse clicks (Remove WS_EX_TRANSPARENT)
				procSetWindowLongW.Call(hwndPtr, gwlExStyle, exStyle|WS_EX_LAYERED)
			} else {
				// Menu Closed = Mouse passes through entirely to game (Add WS_EX_TRANSPARENT)
				procSetWindowLongW.Call(hwndPtr, gwlExStyle, exStyle|WS_EX_LAYERED|WS_EX_TRANSPARENT)
			}
		}

		if now.Sub(lastUpdate).Seconds() >= 0.007 {
			lastUpdate = now

			bMat := reader.ReadBytes(ClientBase+DwViewMatrix, 64)
			if len(bMat) == 64 {
				matrix = make([]float32, 16)
				for i := 0; i < 16; i++ {
					matrix[i] = math.Float32frombits(binary.LittleEndian.Uint32(bMat[i*4 : i*4+4]))
				}
			}

			enemies, localTeam = enumerateEnemies(reader, schema, 64)
		}

		// -------------------------------------------------------------
		// ImGui UI Logic
		// -------------------------------------------------------------
		imgui.NewFrame()

		if showMenu {
			imgui.SetNextWindowSizeV(imgui.Vec2{X: 400, Y: 300}, imgui.ConditionFirstUseEver)
			if imgui.BeginV("CS2 Config Panel (Modern)", nil, imgui.WindowFlagsNone) {
				if imgui.BeginTabBar("Tabs") {

					// ESP Settings Tab
					if imgui.BeginTabItem("Visuals") {
						imgui.Checkbox("Enable ESP", &espEnabled)
						imgui.Separator()

						imgui.Checkbox("Show Teammates", &showTeam)
						imgui.Checkbox("Draw Skeleton (Bones)", &showBones)
						imgui.Checkbox("Draw Player Names", &showNames)

						imgui.EndTabItem()
					}

					// Legitbot Tab
if imgui.BeginTabItem("Legitbot") {
imgui.Checkbox("Enable Legitbot", &legitbotEnabled)
imgui.SliderFloat("Smoothness", &legitbotSmooth, 1.0, 10.0)
imgui.SliderFloat("FOV (Pixels)", &legitbotFov, 10.0, 500.0)
imgui.Checkbox("Draw FOV Circle", &drawFov)
imgui.EndTabItem()
}

// Ragebot Tab
if imgui.BeginTabItem("Ragebot") {
imgui.Checkbox("Enable Ragebot", &ragebotEnabled)
imgui.SliderFloat("FOV (Pixels)", &ragebotFov, 10.0, 500.0)
imgui.Checkbox("pSilent", &psilentEnabled)
if imgui.IsItemHovered() {
imgui.SetTooltip("Note: pSilent automatically aims, shoots, and restores view angles in 1 tick.")
}
imgui.EndTabItem()
}

if imgui.BeginTabItem("Misc") {
imgui.Checkbox("Visibility Check", &visCheckEnabled)
imgui.Checkbox("No Visual Recoil", &noVisualRecoil)
imgui.Checkbox("RCS (Recoil Control)", &rcsEnabled)
imgui.EndTabItem()
}

					// Options Tab
					if imgui.BeginTabItem("Overlay Settings") {
						imgui.Text("Status: Attached to cs2.exe")
						imgui.Text("Overlay Mode: Hardware Accelerated ImGui")
						imgui.Separator()
						imgui.Text("Press [INSERT] to hide menu and play.")
						imgui.EndTabItem()
					}

					imgui.EndTabBar()
				}
			}
			imgui.End()
		}

		imgui.Render()

		// -------------------------------------------------------------
		// ESP 3D Rendering Over Game
		// -------------------------------------------------------------

		
		var bestTarget *EntityData
		var bestDist float64
		if legitbotEnabled {
			bestDist = float64(legitbotFov)
		} else if ragebotEnabled {
			bestDist = float64(ragebotFov)
		} else {
			bestDist = 0.0 // not used
		}

		centerX, centerY := fW/2, fH/2

		if drawFov {
			if legitbotEnabled {
				drawCircle(centerX, centerY, legitbotFov, fW, fH, 32, 1.0, 1.0, 1.0, 1.0)
			} else if ragebotEnabled {
				drawCircle(centerX, centerY, ragebotFov, fW, fH, 32, 1.0, 0.0, 0.0, 1.0)
			}
		}

		if espEnabled && len(matrix) == 16 && len(enemies) > 0 {
			for i := range enemies {
				e := &enemies[i]
				isEnemy := localTeam == 0 || e.TeamValue != localTeam

				headX, headY, headZ := e.X, e.Y, e.Z+70.0
				headSX, headSY, headOk := worldToScreen(headX, headY, headZ, matrix, fW, fH)
				_, footSY, footOk := worldToScreen(e.X, e.Y, e.Z, matrix, fW, fH)

				                                if isEnemy && headOk {
                                        if !visCheckEnabled || e.Spotted {
                                                dist := math.Hypot(float64(headSX-centerX), float64(headSY-centerY))
                                                if dist < bestDist {
                                                        bestDist = dist
                                                        bestTarget = e
                                                }
                                        }
                                }

				if !isEnemy && !showTeam {
					continue
				}

				if showBones && e.BonePtr != 0 {
					boneCache := make(map[uint64][5]float32) // x, y, z, sx, sy

					for _, conn := range boneConnections {
						b1, b2 := conn[0], conn[1]

						// Cache Bone 1
						if _, exists := boneCache[b1]; !exists {
							bP1 := reader.ReadBytes(e.BonePtr+b1*32, 12)
							x1 := math.Float32frombits(binary.LittleEndian.Uint32(bP1[0:4]))
							y1 := math.Float32frombits(binary.LittleEndian.Uint32(bP1[4:8]))
							z1 := math.Float32frombits(binary.LittleEndian.Uint32(bP1[8:12]))
							if math.Abs(float64(x1)) > 0.1 || math.Abs(float64(z1)) > 0.1 {
								if sx, sy, ok := worldToScreen(x1, y1, z1, matrix, fW, fH); ok {
									boneCache[b1] = [5]float32{x1, y1, z1, sx, sy}
								}
							}
						}

						// Cache Bone 2
						if _, exists := boneCache[b2]; !exists {
							bP2 := reader.ReadBytes(e.BonePtr+b2*32, 12)
							x2 := math.Float32frombits(binary.LittleEndian.Uint32(bP2[0:4]))
							y2 := math.Float32frombits(binary.LittleEndian.Uint32(bP2[4:8]))
							z2 := math.Float32frombits(binary.LittleEndian.Uint32(bP2[8:12]))
							if math.Abs(float64(x2)) > 0.1 || math.Abs(float64(z2)) > 0.1 {
								if sx, sy, ok := worldToScreen(x2, y2, z2, matrix, fW, fH); ok {
									boneCache[b2] = [5]float32{x2, y2, z2, sx, sy}
								}
							}
						}

						p1, ok1 := boneCache[b1]
						p2, ok2 := boneCache[b2]
						if ok1 && ok2 {
							dx3D := float64(p2[0] - p1[0])
							dy3D := float64(p2[1] - p1[1])
							dz3D := float64(p2[2] - p1[2])
							dist3D := math.Sqrt(dx3D*dx3D + dy3D*dy3D + dz3D*dz3D)

							if dist3D > 0.1 && dist3D < 150.0 {
								// WHITE BONES
								drawLine(p1[3], p1[4], p2[3], p2[4], fW, fH, 1.5, 1.0, 1.0, 1.0)
							}
						}
					}

					if p, ok := boneCache[6]; ok {
						headSX, headSY, headOk = p[3], p[4], true
					}
				}

				if headOk && footOk {
					h := footSY - headSY
					w := h / 1.6
					if h > 5 && h < fH*2 {
						// BLACK BOX
						drawBox(headSX-(w/2), headSY, w, h, fW, fH, 1.5, 0.0, 0.0, 0.0)
					}

					if showNames && e.Name != "" {
						textX := headSX - (w / 2)
						textY := headSY - 15.0
						if textX >= 0 && textX <= fW && textY >= 0 && textY <= fH {
							drawGLText(textX, textY, e.Name, 0.0, 0.0, 0.0, fW, fH)

							if !isEnemy {
								ndcX := ((textX-8)/fW)*2 - 1
								ndcY := 1 - ((textY+12)/fH)*2
								ndcW := (6.0 / fW) * 2
								ndcH := (6.0 / fH) * 2

								gl.Color4f(0.0, 0.0, 1.0, 1.0)
								gl.Begin(gl.QUADS)
								gl.Vertex2f(ndcX, ndcY)
								gl.Vertex2f(ndcX+ndcW, ndcY)
								gl.Vertex2f(ndcX+ndcW, ndcY-ndcH)
								gl.Vertex2f(ndcX, ndcY-ndcH)
								gl.End()
							}
						}
					}
				}
			}
		}

		if bestTarget != nil {
                        headX, headY, headZ := bestTarget.X, bestTarget.Y, bestTarget.Z+70.0
                        
                        // We must aim using memory writes, because CS2 ignores synthetic mouse_event.
                        localPawn := reader.ReadUint64(ClientBase + DwLocalPlayerPawn)
                        if isProbableUserPointer(localPawn) {
                                localX := reader.ReadFloat32(localPawn + 5048) // m_vOldOrigin X
                                localY := reader.ReadFloat32(localPawn + 5048 + 4) // m_vOldOrigin Y
                                localZ := reader.ReadFloat32(localPawn + 5048 + 8) // m_vOldOrigin Z
                                
                                                                                        viewOffZ := reader.ReadFloat32(localPawn + 3704 + 8) // m_vecViewOffset Z
                                                        eyeZ := localZ + viewOffZ

                                                        var punchPitch, punchYaw float32
																_ = punchPitch
																_ = punchYaw
                                                        if rcsEnabled || noVisualRecoil {
                                                                // m_pAimPunchServices = 5304, m_predictableBaseAngle = 80
                                                                aimPunchPtr := reader.ReadUint64(localPawn + 5304)
                                                                if isProbableUserPointer(aimPunchPtr) {
                                                                        punchPitch = reader.ReadFloat32(aimPunchPtr + 80)
                                                                        punchYaw = reader.ReadFloat32(aimPunchPtr + 80 + 4)
                                                                        
                                                                        if noVisualRecoil {
                                                                                // Zero it out visually
                                                                                reader.WriteFloat32(aimPunchPtr + 80, 0.0)
                                                                                reader.WriteFloat32(aimPunchPtr + 80 + 4, 0.0)
                                                                                
                                                                                // m_pCameraServices = 4672, m_vecCsViewPunchAngle = 72
                                                                                camServices := reader.ReadUint64(localPawn + 4672)
                                                                                if isProbableUserPointer(camServices) {
                                                                                        reader.WriteFloat32(camServices + 72, 0.0)
                                                                                        reader.WriteFloat32(camServices + 72 + 4, 0.0)
                                                                                }
                                                                        }
                                                                }
                                                        }

                                                        relX := headX - localX
                                relY := headY - localY
                                relZ := headZ - eyeZ
                                dist2D := math.Sqrt(float64(relX*relX + relY*relY)) 
                                
                                targetYaw := float32(math.Atan2(float64(relY), float64(relX)) * 180 / math.Pi)
                                targetPitch := float32(-math.Atan2(float64(relZ), dist2D) * 180 / math.Pi)
                                
                                // Clamp pitch
                                if targetPitch > 89.0 { targetPitch = 89.0 }
                                if targetPitch < -89.0 { targetPitch = -89.0 }
                                // Clamp Yaw
                                for targetYaw > 180.0 { targetYaw -= 360.0 }
                                for targetYaw < -180.0 { targetYaw += 360.0 }

                                if ragebotEnabled || (legitbotEnabled && isKeyPressed(VK_LBUTTON)) {
                                        fmt.Printf("Aim on dist=%.2f target={%.1f,%.1f,%.1f} my={%.1f,%.1f,%.1f} p/y={%.2f,%.2f} dwAttack=%d\n", dist2D, headX, headY, headZ, localX, localY, eyeZ, targetPitch, targetYaw, DwForceAttack)
                                }

                                // Legitbot
                                if legitbotEnabled && isKeyPressed(VK_LBUTTON) {
                                        currPitch := reader.ReadFloat32(ClientBase + DwViewAngles)
                                        currYaw := reader.ReadFloat32(ClientBase + DwViewAngles + 4)
                                        
                                        diffPitch := targetPitch - currPitch
                                        diffYaw := targetYaw - currYaw
                                        
                                        for diffYaw > 180.0 { diffYaw -= 360.0 }
                                        for diffYaw < -180.0 { diffYaw += 360.0 }

                                        newPitch := currPitch + (diffPitch / legitbotSmooth)
                                        newYaw := currYaw + (diffYaw / legitbotSmooth)

                                        reader.WriteFloat32(ClientBase+DwViewAngles, newPitch)
                                        reader.WriteFloat32(ClientBase+DwViewAngles+4, newYaw)
                                }

                                // Ragebot
                                if ragebotEnabled {
                                                                                if psilentEnabled {
                                                go func() {
                                                        oldPitch := reader.ReadFloat32(ClientBase + DwViewAngles)
                                                        oldYaw := reader.ReadFloat32(ClientBase + DwViewAngles + 4)

                                                        reader.WriteFloat32(ClientBase+DwViewAngles, targetPitch)
                                                        reader.WriteFloat32(ClientBase+DwViewAngles+4, targetYaw)
                                                        
                                                        if DwForceAttack != 0 {
                                                                reader.WriteInt32(ClientBase + DwForceAttack, 65537) // +attack
                                                        }

                                                        // Wait for tick to register the shot
                                                        time.Sleep(30 * time.Millisecond)
                                                        
                                                        if DwForceAttack != 0 {
                                                                reader.WriteInt32(ClientBase + DwForceAttack, 256) // -attack
                                                        }

                                                        // Restore angles
                                                        reader.WriteFloat32(ClientBase+DwViewAngles, oldPitch)
                                                        reader.WriteFloat32(ClientBase+DwViewAngles+4, oldYaw)
                                                }()
                                        } else {
                                                go func() {
                                                        // Snappy aim write directly to angles
                                                        reader.WriteFloat32(ClientBase+DwViewAngles, targetPitch)
                                                        reader.WriteFloat32(ClientBase+DwViewAngles+4, targetYaw)
                                                        
                                                        if DwForceAttack != 0 {
                                                                reader.WriteInt32(ClientBase + DwForceAttack, 65537)
                                                                time.Sleep(30 * time.Millisecond)
                                                                reader.WriteInt32(ClientBase + DwForceAttack, 256)
                                                        }
                                                }()
                                        }
                                }
                        }
                }

                // Inject ImGui directly into the OpenGL stream after the game ESP lines
		if showMenu {
			renderImGuiDrawData(imgui.RenderedDrawData(), fW, fH)
		}

		window.SwapBuffers()
	}
}
