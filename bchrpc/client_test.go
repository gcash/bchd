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

	certificateFile := filepath.Join(bchutil.AppDataDir("bchd", false), "rpc.cert")
	creds, err := credentials.NewClientTLSFromFile(certificateFile, "localhost")
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := grpc.Dial("localhost:18335", grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	c := pb.NewBchrpcClient(conn)

	blockchainInfo, err := c.GetBlockchainInfo(context.Background(), &pb.GetBlockchainInfoRequest{})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(blockchainInfo)
}
