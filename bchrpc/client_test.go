package bchrpc

import (
	"context"
	"fmt"
	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"path/filepath"
	"testing"
)

func TestDisableLog(t *testing.T) {
	var certificateFile = filepath.Join(bchutil.AppDataDir("bchd", false), "rpc.cert")

	creds, err := credentials.NewClientTLSFromFile(certificateFile, "localhost")
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := grpc.Dial("localhost:8335", grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	c := pb.NewBchrpcClient(conn)

	blockchainInfoResp, err := c.GetBlockchainInfo(context.Background(), &pb.GetBlockchainInfoRequest{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(blockchainInfoResp)
}
