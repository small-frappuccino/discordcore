package discordrpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hugolgst/rich-go/ipc"
)

type rpcClient struct {
	logged bool
}

func (c *rpcClient) Login(appID string) error {
	if c.logged {
		return nil
	}
	if appID == "" {
		return fmt.Errorf("application id is required")
	}

	payload, err := json.Marshal(rpcHandshake{
		Version:  "1",
		ClientID: appID,
	})
	if err != nil {
		return fmt.Errorf("marshal handshake: %w", err)
	}
	if err := ipc.OpenSocket(); err != nil {
		return fmt.Errorf("open rpc socket: %w", err)
	}
	_ = ipc.Send(0, string(payload))
	c.logged = true
	return nil
}

func (c *rpcClient) Logout() {
	if !c.logged {
		return
	}
	_ = ipc.CloseSocket()
	c.logged = false
}

func (c *rpcClient) SetActivity(activity *rpcActivity) error {
	if !c.logged {
		return nil
	}
	payload, err := json.Marshal(rpcFrame{
		Command: "SET_ACTIVITY",
		Args: rpcArgs{
			Pid:      os.Getpid(),
			Activity: activity,
		},
		Nonce: newNonce(),
	})
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}
	_ = ipc.Send(1, string(payload))
	return nil
}

func newNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}

type rpcHandshake struct {
	Version  string `json:"v"`
	ClientID string `json:"client_id"`
}

type rpcFrame struct {
	Command string  `json:"cmd"`
	Args    rpcArgs `json:"args"`
	Nonce   string  `json:"nonce"`
}

type rpcArgs struct {
	Pid      int          `json:"pid"`
	Activity *rpcActivity `json:"activity,omitempty"`
}

type rpcActivity struct {
	Details    string         `json:"details,omitempty"`
	State      string         `json:"state,omitempty"`
	Type       *int           `json:"type,omitempty"`
	URL        string         `json:"url,omitempty"`
	Assets     *rpcAssets     `json:"assets,omitempty"`
	Party      *rpcParty      `json:"party,omitempty"`
	Timestamps *rpcTimestamps `json:"timestamps,omitempty"`
	Buttons    []rpcButton    `json:"buttons,omitempty"`
}

type rpcAssets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

type rpcParty struct {
	ID   string `json:"id,omitempty"`
	Size [2]int `json:"size,omitempty"`
}

type rpcTimestamps struct {
	Start *uint64 `json:"start,omitempty"`
	End   *uint64 `json:"end,omitempty"`
}

type rpcButton struct {
	Label string `json:"label,omitempty"`
	URL   string `json:"url,omitempty"`
}
