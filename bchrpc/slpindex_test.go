package bchrpc

import (
	"context"
	"testing"

	"github.com/gcash/bchd/bchrpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestSlpCallsWithoutSlpIndex ensures the SLP calls return Unavailable
// instead of panicking when the SLP index is not enabled.
func TestSlpCallsWithoutSlpIndex(t *testing.T) {
	s := &GrpcServer{}
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"GetSlpGraphSearch", func() error {
			_, err := s.GetSlpGraphSearch(ctx,
				&pb.GetSlpGraphSearchRequest{Hash: make([]byte, 32)})
			return err
		}},
		{"GetSlpTokenMetadata", func() error {
			_, err := s.GetSlpTokenMetadata(ctx,
				&pb.GetSlpTokenMetadataRequest{TokenIds: [][]byte{make([]byte, 32)}})
			return err
		}},
		{"GetSlpTrustedValidation", func() error {
			_, err := s.GetSlpTrustedValidation(ctx,
				&pb.GetSlpTrustedValidationRequest{
					Queries: []*pb.GetSlpTrustedValidationRequest_Query{{
						PrevOutHash: make([]byte, 32),
					}},
					IncludeGraphsearchCount: true,
				})
			return err
		}},
		{"CheckSlpTransaction", func() error {
			_, err := s.CheckSlpTransaction(ctx,
				&pb.CheckSlpTransactionRequest{})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s panicked: %v", test.name, r)
				}
			}()
			if got := status.Code(test.call()); got != codes.Unavailable {
				t.Fatalf("got code %v, want %v", got, codes.Unavailable)
			}
		})
	}
}
