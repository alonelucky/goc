package client

import (
	"encoding/json"
	"fmt"
	"golang.org/x/term"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/qiniu/goc/v2/pkg/log"
)

// Action provides methods to contact with the covered agent under test
type Action interface {
	ListAgents(bool)
}

const (
	// CoverAgentsListAPI list all the registered agents
	CoverAgentsListAPI = "/v2/rpcagents"
)

type client struct {
	Host   string
	client *http.Client
}

// gocListAgents response of the list request
type gocListAgents struct {
	Items []gocCoveredAgent `json:"items"`
}

// gocCoveredAgent represents a covered client
type gocCoveredAgent struct {
	Id       string `json:"id"`
	RemoteIP string `json:"remoteip"`
	Hostname string `json:"hostname"`
	CmdLine  string `json:"cmdline"`
	Pid      string `json:"pid"`
}

// NewWorker creates a worker to contact with host
func NewWorker(host string) Action {
	_, err := url.ParseRequestURI(host)
	if err != nil {
		log.Fatalf("parse url %s failed, err: %v", host, err)
	}
	return &client{
		Host:   host,
		client: http.DefaultClient,
	}
}

func (c *client) ListAgents(wide bool) {
	u := fmt.Sprintf("%s%s", c.Host, CoverAgentsListAPI)
	_, body, err := c.do("GET", u, "", nil)
	if err != nil && isNetworkError(err) {
		_, body, err = c.do("GET", u, "", nil)
	}
	if err != nil {
		err = fmt.Errorf("goc list failed: %v", err)
		log.Fatalf(err.Error())
	}
	agents := gocListAgents{}
	err = json.Unmarshal(body, &agents)
	if err != nil {
		err = fmt.Errorf("goc list failed: json unmarshal failed: %v", err)
		log.Fatalf(err.Error())
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("   ") // pad with 3 blank spaces
	table.SetNoWhiteSpace(true)
	table.SetReflowDuringAutoWrap(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)
	if wide {
		table.SetHeader([]string{"ID", "REMOTEIP", "HOSTNAME", "PID", "CMD"})
		table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
	} else {
		table.SetHeader([]string{"ID", "REMOTEIP", "CMD"})
		table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
	}
	for _, agent := range agents.Items {
		if wide {
			table.Append([]string{agent.Id, agent.RemoteIP, agent.Hostname, agent.Pid, agent.CmdLine})
		} else {
			preLen := len(agent.Id) + len(agent.RemoteIP) + 9
			table.Append([]string{agent.Id, agent.RemoteIP, getSimpleCmdLine(preLen, agent.CmdLine)})
		}
	}
	table.Render()
	return
}

// getSimpleCmdLine
func getSimpleCmdLine(preLen int, cmdLine string) string {
	pathLen := len(cmdLine)
	width, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil || width <= preLen+16 {
		width = 16 + preLen // show at least 16 words of the command
	}
	if pathLen > width-preLen {
		return cmdLine[:width-preLen]
	}
	return cmdLine
}

func (c *client) do(method, url, contentType string, body io.Reader) (*http.Response, []byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return res, nil, err
	}
	return res, responseBody, nil
}

func isNetworkError(err error) bool {
	if err == io.EOF {
		return true
	}
	_, ok := err.(net.Error)
	return ok
}
