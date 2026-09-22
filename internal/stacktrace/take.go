package stacktrace

import (
	"runtime"
	"strconv"
)

func Take(skip int) string {
	stack := Capture(skip + 1)
	defer stack.Free()

	if stack.Count() == 0 {
		return ""
	}

	buf := make([]byte, 0, 1024)

	appendFrame := func(frame runtime.Frame) {
		if len(buf) > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, frame.Function...)
		buf = append(buf, '\n')
		buf = append(buf, '\t')
		buf = append(buf, frame.File...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(frame.Line), 10)
	}

	// first frame is always included
	frame, more := stack.Next()
	appendFrame(frame)

	// include every frame except the final one
	for more {
		frame, more = stack.Next()
		if !more {
			break
		}
		appendFrame(frame)
	}
	return string(buf)
}
