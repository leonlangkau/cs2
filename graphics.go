package main

import (
	"math"

	"github.com/go-gl/gl/v2.1/gl"
)

func worldToScreen(x, y, z float32, matrix []float32, width, height float32) (float32, float32, bool) {
	w := matrix[12]*x + matrix[13]*y + matrix[14]*z + matrix[15]
	if w < 0.1 {
		return 0, 0, false
	}

	clipX := matrix[0]*x + matrix[1]*y + matrix[2]*z + matrix[3]
	clipY := matrix[4]*x + matrix[5]*y + matrix[6]*z + matrix[7]

	ndcX := clipX / w
	ndcY := clipY / w

	halfW := width / 2.0
	halfH := height / 2.0

	screenX := halfW*ndcX + (ndcX + halfW)
	screenY := -halfH*ndcY + halfH

	return screenX, screenY, true
}

func drawBox(x, y, w, h float32, screenW, screenH float32, thickness float32, r, g, b float32) {
	ndcX := (x/screenW)*2 - 1
	ndcY := 1 - (y/screenH)*2
	ndcW := (w / screenW) * 2
	ndcH := (h / screenH) * 2

	gl.LineWidth(thickness)
	gl.Color4f(r, g, b, 1.0)
	gl.Begin(gl.LINE_LOOP)
	gl.Vertex2f(ndcX, ndcY)
	gl.Vertex2f(ndcX+ndcW, ndcY)
	gl.Vertex2f(ndcX+ndcW, ndcY-ndcH)
	gl.Vertex2f(ndcX, ndcY-ndcH)
	gl.End()
}

func drawLine(x1, y1, x2, y2, screenW, screenH float32, thickness float32, r, g, b float32) {
	n1x := (x1/screenW)*2 - 1
	n1y := 1 - (y1/screenH)*2
	n2x := (x2/screenW)*2 - 1
	n2y := 1 - (y2/screenH)*2

	gl.LineWidth(thickness)
	gl.Color4f(r, g, b, 1.0)
	gl.Begin(gl.LINES)
	gl.Vertex2f(n1x, n1y)
	gl.Vertex2f(n2x, n2y)
	gl.End()
}

func drawCircle(centerX, centerY, radius, screenW, screenH float32, segments int, thickness float32, r, g, b float32) {
	gl.LineWidth(thickness)
	gl.Color4f(r, g, b, 1.0)
	gl.Begin(gl.LINE_LOOP)

	for i := 0; i < segments; i++ {
		theta := float64(2.0 * float32(i) * 3.1415926 / float32(segments))

		offsetX := radius * float32(math.Cos(theta))
		offsetY := radius * float32(math.Sin(theta))

		ndcX := ((centerX+offsetX)/screenW)*2 - 1
		ndcY := 1 - ((centerY+offsetY)/screenH)*2

		gl.Vertex2f(ndcX, ndcY)
	}
	gl.End()
}
