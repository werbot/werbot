package connectcli

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

func TestTargetPort(t *testing.T) {
	one := []sshdgram.Port{{Port: 5432}}
	p, err := targetPort(Spec{Resource: "pg"}, one)
	require.NoError(t, err)
	assert.Equal(t, uint16(5432), p)
	p, err = targetPort(Spec{Resource: "pg"}, []sshdgram.Port{{Port: 5432, EndPort: 5432}})
	require.NoError(t, err)
	assert.Equal(t, uint16(5432), p)
	p, err = targetPort(Spec{Resource: "web", TargetPort: 8001}, []sshdgram.Port{{Port: 8000, EndPort: 8002}})
	require.NoError(t, err)
	assert.Equal(t, uint16(8001), p)

	_, err = targetPort(Spec{Resource: "web"}, []sshdgram.Port{{Port: 8000, EndPort: 8002}, {Port: 9000}})
	assert.EqualError(t, err, "resource web has several ports (8000-8002, 9000); use web:<port>")
	_, err = targetPort(Spec{Resource: "any"}, nil)
	assert.EqualError(t, err, "resource any has no ports; use any:<port>")
}

func TestListenBusy(t *testing.T) {
	for _, udp := range []bool{false, true} {
		port := freePortNum(t)
		ln, pc, err := listen(udp, port, "pg")
		require.NoError(t, err)
		if udp {
			assert.Nil(t, ln)
			assert.Equal(t, "127.0.0.1:"+strconv.Itoa(int(port)), pc.LocalAddr().String())
		} else {
			assert.Nil(t, pc)
			assert.Equal(t, "127.0.0.1:"+strconv.Itoa(int(port)), ln.Addr().String())
		}
		_, _, err = listen(udp, port, "pg")
		assert.Regexp(t, `^local port `+strconv.Itoa(int(port))+` is busy; use \d+:pg$`, err, "udp=%v", udp)
		if udp {
			_ = pc.Close()
		} else {
			_ = ln.Close()
		}
	}
}
