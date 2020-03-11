/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"errors"
	"testing"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	ob "github.com/hyperledger/fabric-protos-go/orderer"
	eb "github.com/hyperledger/fabric-protos-go/orderer/etcdraft"
	. "github.com/onsi/gomega"
)

func TestNewOrdererGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ordererType           string
		numOrdererGroupValues int
	}{
		{ordererType: ConsensusTypeSolo, numOrdererGroupValues: 5},
		{ordererType: ConsensusTypeEtcdRaft, numOrdererGroupValues: 5},
		{ordererType: ConsensusTypeKafka, numOrdererGroupValues: 6},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.ordererType, func(t *testing.T) {
			t.Parallel()

			gt := NewGomegaWithT(t)

			ordererConf := baseOrderer()
			ordererConf.OrdererType = tt.ordererType

			ordererGroup, err := newOrdererGroup(ordererConf)
			gt.Expect(err).NotTo(HaveOccurred())

			// OrdererGroup checks
			gt.Expect(len(ordererGroup.Groups)).To(Equal(1))
			gt.Expect(ordererGroup.Groups["OrdererOrg"]).NotTo(BeNil())
			gt.Expect(len(ordererGroup.Values)).To(Equal(tt.numOrdererGroupValues))
			gt.Expect(ordererGroup.Values[BatchSizeKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Values[BatchTimeoutKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Values[ChannelRestrictionsKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Values[CapabilitiesKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Values[ConsensusTypeKey]).NotTo(BeNil())
			var consensusType ob.ConsensusType
			err = proto.Unmarshal(ordererGroup.Values[ConsensusTypeKey].Value, &consensusType)
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(consensusType.Type).To(Equal(tt.ordererType))
			gt.Expect(len(ordererGroup.Policies)).To(Equal(4))
			gt.Expect(ordererGroup.Policies[AdminsPolicyKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Policies[ReadersPolicyKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Policies[WritersPolicyKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Policies[BlockValidationPolicyKey]).NotTo(BeNil())

			// OrdererOrgGroup checks
			gt.Expect(len(ordererGroup.Groups["OrdererOrg"].Groups)).To(Equal(0))
			gt.Expect(len(ordererGroup.Groups["OrdererOrg"].Values)).To(Equal(2))
			gt.Expect(ordererGroup.Groups["OrdererOrg"].Values[MSPKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Groups["OrdererOrg"].Values[EndpointsKey]).NotTo(BeNil())
			gt.Expect(len(ordererGroup.Groups["OrdererOrg"].Policies)).To(Equal(4))
			gt.Expect(ordererGroup.Groups["OrdererOrg"].Policies[AdminsPolicyKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Groups["OrdererOrg"].Policies[ReadersPolicyKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Groups["OrdererOrg"].Policies[WritersPolicyKey]).NotTo(BeNil())
			gt.Expect(ordererGroup.Groups["OrdererOrg"].Policies[EndorsementPolicyKey]).NotTo(BeNil())
		})
	}
}

func TestNewOrdererGroupFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testName   string
		ordererMod func(*Orderer)
		err        error
	}{
		{
			testName: "When orderer group policy is empty",
			ordererMod: func(o *Orderer) {
				o.Policies = nil
			},
			err: errors.New("no policies defined"),
		},
		{
			testName: "When marshalling etcdraft metadata for orderer group",
			ordererMod: func(o *Orderer) {
				o.OrdererType = ConsensusTypeEtcdRaft
				o.EtcdRaft = eb.ConfigMetadata{
					Consenters: []*eb.Consenter{
						{
							Host:          "node-1.example.com",
							Port:          7050,
							ClientTlsCert: []byte("testdata/tls-client-1.pem"),
							ServerTlsCert: []byte("testdata/tls-server-1.pem"),
						},
						{
							Host:          "node-2.example.com",
							Port:          7050,
							ClientTlsCert: []byte("testdata/tls-client-2.pem"),
							ServerTlsCert: []byte("testdata/tls-server-2.pem"),
						},
						{
							Host:          "node-3.example.com",
							Port:          7050,
							ClientTlsCert: []byte("testdata/tls-client-3.pem"),
							ServerTlsCert: []byte("testdata/tls-server-3.pem"),
						},
					},
				}
			},
			err: errors.New("marshalling etcdraft metadata for orderer type 'etcdraft': " +
				"cannot load client cert for consenter node-1.example.com:7050: open testdata/tls-client-1.pem: " +
				"no such file or directory"),
		},
		{
			testName: "When orderer type is unknown",
			ordererMod: func(o *Orderer) {
				o.OrdererType = "ConsensusTypeGreen"
			},
			err: errors.New("unknown orderer type 'ConsensusTypeGreen'"),
		},
		{
			testName: "When adding policies to orderer org group",
			ordererMod: func(o *Orderer) {
				o.Organizations[0].Policies = nil
			},
			err: errors.New("org group 'OrdererOrg': no policies defined"),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()

			gt := NewGomegaWithT(t)

			ordererConf := baseOrderer()
			tt.ordererMod(&ordererConf)

			ordererGroup, err := newOrdererGroup(ordererConf)
			gt.Expect(err).To(MatchError(tt.err))
			gt.Expect(ordererGroup).To(BeNil())
		})
	}
}

func TestUpdateOrdererConfiguration(t *testing.T) {
	t.Parallel()

	gt := NewGomegaWithT(t)

	baseOrdererConf := baseOrderer()

	ordererGroup, err := newOrdererGroup(baseOrdererConf)
	gt.Expect(err).NotTo(HaveOccurred())

	originalOrdererAddresses, err := proto.Marshal(&cb.OrdererAddresses{
		Addresses: baseOrdererConf.Addresses,
	})
	gt.Expect(err).NotTo(HaveOccurred())

	imp, err := implicitMetaFromString(baseOrdererConf.Policies[AdminsPolicyKey].Rule)
	gt.Expect(err).NotTo(HaveOccurred())

	originalAdminsPolicy, err := proto.Marshal(imp)
	gt.Expect(err).NotTo(HaveOccurred())

	config := &cb.Config{
		ChannelGroup: &cb.ConfigGroup{
			Groups: map[string]*cb.ConfigGroup{
				"Orderer": ordererGroup,
			},
			Values: map[string]*cb.ConfigValue{
				OrdererAddressesKey: {
					Value:     originalOrdererAddresses,
					ModPolicy: AdminsPolicyKey,
				},
			},
			Policies: map[string]*cb.ConfigPolicy{
				AdminsPolicyKey: {
					Policy: &cb.Policy{
						Type:  int32(cb.Policy_IMPLICIT_META),
						Value: originalAdminsPolicy,
					},
					ModPolicy: AdminsPolicyKey,
				},
			},
		},
	}

	updatedOrdererConf := baseOrdererConf

	// Modify MaxMessageCount, Addresses, and ConesnsusType to etcdraft
	updatedOrdererConf.BatchSize.MaxMessageCount = 10000
	updatedOrdererConf.Addresses = []string{"newhost:345"}
	updatedOrdererConf.OrdererType = ConsensusTypeEtcdRaft

	err = UpdateOrdererConfiguration(config, updatedOrdererConf)
	gt.Expect(err).NotTo(HaveOccurred())

	// Expected OrdererValues
	expectedCapabilities, err := proto.Marshal(&cb.Capabilities{
		Capabilities: map[string]*cb.Capability{
			"V1_3": {},
		},
	})
	gt.Expect(err).NotTo(HaveOccurred())

	expectedConsensusType, err := proto.Marshal(&ob.ConsensusType{
		Type:     ConsensusTypeEtcdRaft,
		Metadata: []byte{},
	})
	gt.Expect(err).NotTo(HaveOccurred())

	expectedBatchSize, err := proto.Marshal(&ob.BatchSize{
		MaxMessageCount:   10000,
		AbsoluteMaxBytes:  100,
		PreferredMaxBytes: 100,
	})
	gt.Expect(err).NotTo(HaveOccurred())

	expectedBatchTimeout, err := proto.Marshal(&ob.BatchTimeout{
		Timeout: "0s",
	})
	gt.Expect(err).NotTo(HaveOccurred())

	expectedChannelRestrictions, err := proto.Marshal(&ob.ChannelRestrictions{
		MaxCount: 0,
	})
	gt.Expect(err).NotTo(HaveOccurred())

	expectedOrdererValues := map[string]*cb.ConfigValue{
		CapabilitiesKey:        {Value: expectedCapabilities, ModPolicy: AdminsPolicyKey},
		ConsensusTypeKey:       {Value: expectedConsensusType, ModPolicy: AdminsPolicyKey},
		BatchSizeKey:           {Value: expectedBatchSize, ModPolicy: AdminsPolicyKey},
		BatchTimeoutKey:        {Value: expectedBatchTimeout, ModPolicy: AdminsPolicyKey},
		ChannelRestrictionsKey: {Value: expectedChannelRestrictions, ModPolicy: AdminsPolicyKey},
	}

	// Expected OrdererAddresses
	expectedOrdererAddresses, err := proto.Marshal(&cb.OrdererAddresses{
		Addresses: []string{"newhost:345"},
	})
	gt.Expect(err).NotTo(HaveOccurred())

	gt.Expect(config.ChannelGroup.Groups["Orderer"].Values).To(Equal(expectedOrdererValues))
	gt.Expect(config.ChannelGroup.Values[OrdererAddressesKey].Value).To(Equal(expectedOrdererAddresses))
}

func baseOrderer() Orderer {
	return Orderer{
		Policies:    ordererStandardPolicies(),
		OrdererType: ConsensusTypeSolo,
		Organizations: []Organization{
			{
				Name:     "OrdererOrg",
				ID:       "OrdererOrgMSP",
				Policies: orgStandardPolicies(),
				OrdererEndpoints: []string{
					"localhost:123",
				},
				MSP: baseMSP(),
			},
		},
		Capabilities: map[string]bool{
			"V1_3": true,
		},
		BatchSize: BatchSize{
			MaxMessageCount:   100,
			AbsoluteMaxBytes:  100,
			PreferredMaxBytes: 100,
		},
		Addresses: []string{"localhost:123"},
	}
}
