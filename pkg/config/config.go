/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"bytes"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	cb "github.com/hyperledger/fabric-protos-go/common"
	mb "github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric/common/policydsl"
)

// Channel encapsulates basic information for a channel config profile.
type Channel struct {
	Consortium   string
	Application  *Application
	Orderer      *Orderer
	Consortiums  []*Consortium
	Capabilities map[string]bool
	Policies     map[string]*Policy
	ChannelID    string
}

// Policy encodes a channel config policy.
type Policy struct {
	Type string
	Rule string
}

// Resources encodes the application-level resources configuration needed to
// seed the resource tree.
type Resources struct {
	DefaultModPolicy string
}

// Organization encodes the organization-level configuration needed in
// config transactions.
type Organization struct {
	Name     string
	ID       string
	Policies map[string]*Policy

	AnchorPeers      []*AnchorPeer
	OrdererEndpoints []string

	// SkipAsForeign indicates that this org definition is actually unknown to this
	// instance of the tool, so, parsing of this org's parameters should be ignored
	SkipAsForeign bool
}

type standardConfigValue struct {
	key   string
	value proto.Message
}

type standardConfigPolicy struct {
	key   string
	value *cb.Policy
}

// NewCreateChannelTx creates a create channel tx using the provided channel config.
func NewCreateChannelTx(channelConfig *Channel, mspConfig *mb.FabricMSPConfig) (*cb.Envelope, error) {
	var err error

	if channelConfig == nil {
		return nil, errors.New("channel config is required")
	}

	channelID := channelConfig.ChannelID

	if channelID == "" {
		return nil, errors.New("profile's channel ID is required")
	}

	config, err := proto.Marshal(mspConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal msp config: %v", err)
	}

	// mspconf defaults type to FABRIC which implements an X.509 based provider
	mspconf := &mb.MSPConfig{
		Config: config,
	}

	ct, err := defaultConfigTemplate(channelConfig, mspconf)
	if err != nil {
		return nil, fmt.Errorf("creating default config template: %v", err)
	}

	newChannelConfigUpdate, err := newChannelCreateConfigUpdate(channelID, channelConfig, ct, mspconf)
	if err != nil {
		return nil, fmt.Errorf("creating channel create config update: %v", err)
	}

	configUpdate, err := proto.Marshal(newChannelConfigUpdate)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling new channel config update: %v", err)
	}

	newConfigUpdateEnv := &cb.ConfigUpdateEnvelope{
		ConfigUpdate: configUpdate,
	}

	env, err := newEnvelope(cb.HeaderType_CONFIG_UPDATE, channelID, newConfigUpdateEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create envelope: %v", err)
	}

	return env, nil
}

// SignConfigUpdate signs the given configuration update with a
// specific signing identity.
func SignConfigUpdate(configUpdate *cb.ConfigUpdate, signingIdentity *SigningIdentity) (*cb.ConfigSignature, error) {
	signatureHeader, err := signatureHeader(signingIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature header: %v", err)
	}

	header, err := proto.Marshal(signatureHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signature header: %v", err)
	}

	configSignature := &cb.ConfigSignature{
		SignatureHeader: header,
	}

	configUpdateBytes, err := proto.Marshal(configUpdate)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config update: %v", err)
	}

	configSignature.Signature, err = signingIdentity.Sign(rand.Reader, concatenateBytes(configSignature.SignatureHeader, configUpdateBytes), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to sign config update: %v", err)
	}

	return configSignature, nil
}

func signatureHeader(signingIdentity *SigningIdentity) (*cb.SignatureHeader, error) {
	buffer := bytes.NewBuffer(nil)
	err := pem.Encode(buffer, &pem.Block{Type: "CERTIFICATE", Bytes: signingIdentity.Certificate.Raw})
	if err != nil {
		return nil, fmt.Errorf("pem encode: %v", err)
	}
	idBytes, err := proto.Marshal(&mb.SerializedIdentity{Mspid: signingIdentity.MSPID, IdBytes: buffer.Bytes()})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal serialized identity: %v", err)
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}

	return &cb.SignatureHeader{
		Creator: idBytes,
		Nonce:   nonce,
	}, nil
}

// newNonce generates a 24-byte nonce using the crypto/rand package.
func newNonce() ([]byte, error) {
	nonce := make([]byte, 24)

	_, err := rand.Read(nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to get random bytes: %v", err)
	}

	return nonce, nil
}

// newChannelGroup defines the root of the channel configuration.
func newChannelGroup(channelConfig *Channel, mspConfig *mb.MSPConfig) (*cb.ConfigGroup, error) {
	var err error

	channelGroup := newConfigGroup()

	if err = addPolicies(channelGroup, channelConfig.Policies, AdminsPolicyKey); err != nil {
		return nil, fmt.Errorf("failed to add policies to channel group: %v", err)
	}

	err = addValue(channelGroup, hashingAlgorithmValue(), AdminsPolicyKey)
	if err != nil {
		return nil, err
	}

	err = addValue(channelGroup, blockDataHashingStructureValue(), AdminsPolicyKey)
	if err != nil {
		return nil, err
	}

	if channelConfig.Orderer != nil && len(channelConfig.Orderer.Addresses) > 0 {
		err = addValue(channelGroup, ordererAddressesValue(channelConfig.Orderer.Addresses), ordererAdminsPolicyName)
		if err != nil {
			return nil, err
		}
	}

	if channelConfig.Consortium != "" {
		err = addValue(channelGroup, consortiumValue(channelConfig.Consortium), AdminsPolicyKey)
		if err != nil {
			return nil, err
		}
	}

	if len(channelConfig.Capabilities) > 0 {
		err = addValue(channelGroup, capabilitiesValue(channelConfig.Capabilities), AdminsPolicyKey)
		if err != nil {
			return nil, err
		}
	}

	if channelConfig.Orderer != nil {
		channelGroup.Groups[OrdererGroupKey], err = NewOrdererGroup(channelConfig.Orderer, mspConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create orderer group: %v", err)
		}
	}

	if channelConfig.Application != nil {
		channelGroup.Groups[ApplicationGroupKey], err = NewApplicationGroup(channelConfig.Application, mspConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create application group: %v", err)
		}
	}

	if channelConfig.Consortiums != nil {
		channelGroup.Groups[ConsortiumsGroupKey], err = NewConsortiumsGroup(channelConfig.Consortiums, mspConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create consortiums group: %v", err)
		}
	}

	channelGroup.ModPolicy = AdminsPolicyKey

	return channelGroup, nil
}

// hashingAlgorithmValue returns the only currently valid hashing algorithm.
// It is a value for the /Channel group.
func hashingAlgorithmValue() *standardConfigValue {
	return &standardConfigValue{
		key: HashingAlgorithmKey,
		value: &cb.HashingAlgorithm{
			Name: defaultHashingAlgorithm,
		},
	}
}

// blockDataHashingStructureValue returns the only currently valid block data hashing structure.
// It is a value for the /Channel group.
func blockDataHashingStructureValue() *standardConfigValue {
	return &standardConfigValue{
		key: BlockDataHashingStructureKey,
		value: &cb.BlockDataHashingStructure{
			Width: defaultBlockDataHashingStructureWidth,
		},
	}
}

// addValue adds a *cb.ConfigValue to the passed *cb.ConfigGroup's Values map.
func addValue(cg *cb.ConfigGroup, value *standardConfigValue, modPolicy string) error {
	v, err := proto.Marshal(value.value)
	if err != nil {
		return fmt.Errorf("marshalling standard config value '%s': %v", value.key, err)
	}

	cg.Values[value.key] = &cb.ConfigValue{
		Value:     v,
		ModPolicy: modPolicy,
	}

	return nil
}

// addPolicies adds *cb.ConfigPolicies to the passed *cb.ConfigGroup's Policies map.
// TODO: evaluate if modPolicy actually needs to be passed in if all callers pass AdminsPolicyKey.
func addPolicies(cg *cb.ConfigGroup, policyMap map[string]*Policy, modPolicy string) error {
	switch {
	case policyMap == nil:
		return errors.New("no policies defined")
	case policyMap[AdminsPolicyKey] == nil:
		return errors.New("no Admins policy defined")
	case policyMap[ReadersPolicyKey] == nil:
		return errors.New("no Readers policy defined")
	case policyMap[WritersPolicyKey] == nil:
		return errors.New("no Writers policy defined")
	}

	for policyName, policy := range policyMap {
		switch policy.Type {
		case ImplicitMetaPolicyType:
			imp, err := implicitMetaFromString(policy.Rule)
			if err != nil {
				return fmt.Errorf("invalid implicit meta policy rule: '%s': %v", policy.Rule, err)
			}

			implicitMetaPolicy, err := proto.Marshal(imp)
			if err != nil {
				return fmt.Errorf("marshalling implicit meta policy: %v", err)
			}

			cg.Policies[policyName] = &cb.ConfigPolicy{
				ModPolicy: modPolicy,
				Policy: &cb.Policy{
					Type:  int32(cb.Policy_IMPLICIT_META),
					Value: implicitMetaPolicy,
				},
			}
		case SignaturePolicyType:
			sp, err := policydsl.FromString(policy.Rule)
			if err != nil {
				return fmt.Errorf("invalid signature policy rule: '%s': %v", policy.Rule, err)
			}

			signaturePolicy, err := proto.Marshal(sp)
			if err != nil {
				return fmt.Errorf("marshalling signature policy: %v", err)
			}

			cg.Policies[policyName] = &cb.ConfigPolicy{
				ModPolicy: modPolicy,
				Policy: &cb.Policy{
					Type:  int32(cb.Policy_SIGNATURE),
					Value: signaturePolicy,
				},
			}
		default:
			return fmt.Errorf("unknown policy type: %s", policy.Type)
		}
	}

	return nil
}

// implicitMetaFromString parses a *cb.ImplicitMetaPolicy from an input string.
func implicitMetaFromString(input string) (*cb.ImplicitMetaPolicy, error) {
	args := strings.Split(input, " ")
	if len(args) != 2 {
		return nil, fmt.Errorf("expected two space separated tokens, but got %d", len(args))
	}

	res := &cb.ImplicitMetaPolicy{
		SubPolicy: args[1],
	}

	switch args[0] {
	case cb.ImplicitMetaPolicy_ANY.String():
		res.Rule = cb.ImplicitMetaPolicy_ANY
	case cb.ImplicitMetaPolicy_ALL.String():
		res.Rule = cb.ImplicitMetaPolicy_ALL
	case cb.ImplicitMetaPolicy_MAJORITY.String():
		res.Rule = cb.ImplicitMetaPolicy_MAJORITY
	default:
		return nil, fmt.Errorf("unknown rule type '%s', expected ALL, ANY, or MAJORITY", args[0])
	}

	return res, nil
}

// ordererAddressesValue returns the a config definition for the orderer addresses.
// It is a value for the /Channel group.
func ordererAddressesValue(addresses []string) *standardConfigValue {
	return &standardConfigValue{
		key: OrdererAddressesKey,
		value: &cb.OrdererAddresses{
			Addresses: addresses,
		},
	}
}

// capabilitiesValue returns the config definition for a a set of capabilities.
// It is a value for the /Channel/Orderer, Channel/Application/, and /Channel groups.
func capabilitiesValue(capabilities map[string]bool) *standardConfigValue {
	c := &cb.Capabilities{
		Capabilities: make(map[string]*cb.Capability),
	}

	for capability, required := range capabilities {
		if !required {
			continue
		}

		c.Capabilities[capability] = &cb.Capability{}
	}

	return &standardConfigValue{
		key:   CapabilitiesKey,
		value: c,
	}
}

// mspValue returns the config definition for an MSP.
// It is a value for the /Channel/Orderer/*, /Channel/Application/*, and /Channel/Consortiums/*/*/* groups.
func mspValue(mspDef *mb.MSPConfig) *standardConfigValue {
	return &standardConfigValue{
		key:   MSPKey,
		value: mspDef,
	}
}

// defaultConfigTemplate generates a config template based on the assumption that
// the input profile is a channel creation template and no system channel context
// is available.
func defaultConfigTemplate(channelConfig *Channel, mspConfig *mb.MSPConfig) (*cb.ConfigGroup, error) {
	channelGroup, err := newChannelGroup(channelConfig, mspConfig)
	if err != nil {
		return nil, err
	}

	if _, ok := channelGroup.Groups[ApplicationGroupKey]; !ok {
		return nil, errors.New("channel template config must contain an application section")
	}

	channelGroup.Groups[ApplicationGroupKey].Values = nil
	channelGroup.Groups[ApplicationGroupKey].Policies = nil

	return channelGroup, nil
}

// newChannelCreateConfigUpdate generates a ConfigUpdate which can be sent to the orderer to create a new channel.
// Optionally, the channel group of the ordering system channel may be passed in, and the resulting ConfigUpdate
// will extract the appropriate versions from this file.
func newChannelCreateConfigUpdate(channelID string, channelConfig *Channel, templateConfig *cb.ConfigGroup,
	mspConfig *mb.MSPConfig) (*cb.ConfigUpdate, error) {
	newChannelGroup, err := newChannelGroup(channelConfig, mspConfig)
	if err != nil {
		return nil, err
	}

	updt, err := computeConfigUpdate(&cb.Config{ChannelGroup: templateConfig}, &cb.Config{ChannelGroup: newChannelGroup})
	if err != nil {
		return nil, fmt.Errorf("computing update: %v", err)
	}

	wsValue, err := proto.Marshal(&cb.Consortium{
		Name: channelConfig.Consortium,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling consortium: %v", err)
	}

	// Add the consortium name to create the channel for into the write set as required
	updt.ChannelId = channelID
	updt.ReadSet.Values[ConsortiumKey] = &cb.ConfigValue{Version: 0}
	updt.WriteSet.Values[ConsortiumKey] = &cb.ConfigValue{
		Version: 0,
		Value:   wsValue,
	}

	return updt, nil
}

// newConfigGroup creates an empty *cb.ConfigGroup.
func newConfigGroup() *cb.ConfigGroup {
	return &cb.ConfigGroup{
		Groups:   make(map[string]*cb.ConfigGroup),
		Values:   make(map[string]*cb.ConfigValue),
		Policies: make(map[string]*cb.ConfigPolicy),
	}
}

// newEnvelope creates an unsigned envelope of type txType using with the marshalled
// cb.ConfigGroupEnvelope proto message.
func newEnvelope(
	txType cb.HeaderType,
	channelID string,
	dataMsg proto.Message,
) (*cb.Envelope, error) {
	payloadChannelHeader := channelHeader(txType, msgVersion, channelID, epoch)
	payloadSignatureHeader := &cb.SignatureHeader{}

	data, err := proto.Marshal(dataMsg)
	if err != nil {
		return nil, fmt.Errorf("marshalling envelope data: %v", err)
	}

	payloadHeader, err := payloadHeader(payloadChannelHeader, payloadSignatureHeader)
	if err != nil {
		return nil, fmt.Errorf("making payload header: %v", err)
	}

	paylBytes, err := proto.Marshal(
		&cb.Payload{
			Header: payloadHeader,
			Data:   data,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("marshalling payload: %v", err)
	}

	env := &cb.Envelope{
		Payload: paylBytes,
	}

	return env, nil
}

// makeChannelHeader creates a ChannelHeader.
func channelHeader(headerType cb.HeaderType, version int32, channelID string, epoch uint64) *cb.ChannelHeader {
	return &cb.ChannelHeader{
		Type:    int32(headerType),
		Version: version,
		Timestamp: &timestamp.Timestamp{
			Seconds: ptypes.TimestampNow().GetSeconds(),
		},
		ChannelId: channelID,
		Epoch:     epoch,
	}
}

// makePayloadHeader creates a Payload Header.
func payloadHeader(ch *cb.ChannelHeader, sh *cb.SignatureHeader) (*cb.Header, error) {
	channelHeader, err := proto.Marshal(ch)
	if err != nil {
		return nil, fmt.Errorf("marshalling channel header: %v", err)
	}

	signatureHeader, err := proto.Marshal(sh)
	if err != nil {
		return nil, fmt.Errorf("marshalling signature header: %v", err)
	}

	return &cb.Header{
		ChannelHeader:   channelHeader,
		SignatureHeader: signatureHeader,
	}, nil
}

// ComputeUpdate computes the config update from a base and modified config transaction.
func ComputeUpdate(baseConfig, updatedConfig *cb.Config, channelID string) (*cb.ConfigUpdate, error) {
	if channelID == "" {
		return nil, errors.New("channel ID is required")
	}

	updt, err := computeConfigUpdate(baseConfig, updatedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to compute update: %v", err)
	}

	updt.ChannelId = channelID

	return updt, nil
}

// concatenateBytes combines multiple arrays of bytes, for signatures or digests
// over multiple fields.
func concatenateBytes(data ...[]byte) []byte {
	var res []byte
	for i := range data {
		res = append(res, data[i]...)
	}

	return res
}

// newOrgConfigGroup returns an config group for a organization.
// It defines the crypto material for the organization (its MSP).
// It sets the mod_policy of all elements to "Admins".
func newOrgConfigGroup(org *Organization, mspConfig *mb.MSPConfig) (*cb.ConfigGroup, error) {
	var err error

	orgGroup := newConfigGroup()
	orgGroup.ModPolicy = AdminsPolicyKey

	if org.SkipAsForeign {
		return orgGroup, nil
	}

	if err = addPolicies(orgGroup, org.Policies, AdminsPolicyKey); err != nil {
		return nil, err
	}

	err = addValue(orgGroup, mspValue(mspConfig), AdminsPolicyKey)
	if err != nil {
		return nil, err
	}

	if len(org.OrdererEndpoints) > 0 {
		err = addValue(orgGroup, endpointsValue(org.OrdererEndpoints), AdminsPolicyKey)
		if err != nil {
			return nil, err
		}
	}

	return orgGroup, nil
}

// CreateSignedConfigUpdateEnvelope creates a signed configuration update envelope.
func CreateSignedConfigUpdateEnvelope(configUpdate *cb.ConfigUpdate, signingIdentity *SigningIdentity,
	signatures ...*cb.ConfigSignature) (*cb.Envelope, error) {
	update, err := proto.Marshal(configUpdate)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config update: %v", err)
	}

	configUpdateEnvelope := &cb.ConfigUpdateEnvelope{
		ConfigUpdate: update,
		Signatures:   signatures,
	}

	signedEnvelope, err := createSignedEnvelopeWithTLSBinding(cb.HeaderType_CONFIG_UPDATE, configUpdate.ChannelId,
		signingIdentity, configUpdateEnvelope)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed config update envelope: %v", err)
	}

	return signedEnvelope, nil
}

// CreateSignedEnvelopeWithTLSBinding creates a signed envelope of the desired
// type, with marshaled dataMsg and signs it. It also includes a TLS cert hash
// into the channel header
func createSignedEnvelopeWithTLSBinding(
	txType cb.HeaderType,
	channelID string,
	signingIdentity *SigningIdentity,
	envelope proto.Message,
) (*cb.Envelope, error) {
	channelHeader := &cb.ChannelHeader{
		Type: int32(txType),
		Timestamp: &timestamp.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   0,
		},
		ChannelId: channelID,
	}

	signatureHeader, err := signatureHeader(signingIdentity)
	if err != nil {
		return nil, fmt.Errorf("creating signature header: %v", err)
	}

	cHeader, err := proto.Marshal(channelHeader)
	if err != nil {
		return nil, fmt.Errorf("marshalling channel header: %s", err)
	}

	sHeader, err := proto.Marshal(signatureHeader)
	if err != nil {
		return nil, fmt.Errorf("marshalling signature header: %s", err)
	}

	data, err := proto.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshalling config update envelope: %s", err)
	}

	payload := &cb.Payload{
		Header: &cb.Header{
			ChannelHeader:   cHeader,
			SignatureHeader: sHeader,
		},
		Data: data,
	}

	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling payload: %s", err)
	}

	sig, err := signingIdentity.Sign(rand.Reader, payloadBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("signing envelope's payload: %v", err)
	}

	env := &cb.Envelope{
		Payload:   payloadBytes,
		Signature: sig,
	}

	return env, nil
}

// AddOrgToConsortium adds an org definition to a named consortium in a given
// channel configuration.
func AddOrgToConsortium(org *Organization, consortium, channelID string, config *cb.Config, mspConfig *mb.MSPConfig) (*cb.ConfigUpdate, error) {
	if org == nil {
		return nil, errors.New("organization is empty")
	}
	if consortium == "" {
		return nil, errors.New("consortium is empty")
	}

	updatedConfig := proto.Clone(config).(*cb.Config)

	consortiumsGroup, ok := updatedConfig.ChannelGroup.Groups[ConsortiumsGroupKey]
	if !ok {
		return nil, errors.New("consortiums group does not exist")
	}

	consortiumGroup, ok := consortiumsGroup.Groups[consortium]
	if !ok {
		return nil, fmt.Errorf("consortium '%s' does not exist", consortium)
	}

	orgGroup, err := newConsortiumOrgGroup(org, mspConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consortium org: %v", err)
	}

	if consortiumGroup.Groups == nil {
		consortiumGroup.Groups = map[string]*cb.ConfigGroup{}
	}
	consortiumGroup.Groups[org.Name] = orgGroup

	configUpdate, err := ComputeUpdate(config, updatedConfig, channelID)
	if err != nil {
		return nil, err
	}

	return configUpdate, nil
}
