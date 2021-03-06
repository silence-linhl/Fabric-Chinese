
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
/*
版权所有IBM公司。保留所有权利。

SPDX许可证标识符：Apache-2.0
**/


package config_test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/configtx/test"
	"github.com/hyperledger/fabric/common/tools/configtxgen/encoder"
	genesisconfig "github.com/hyperledger/fabric/common/tools/configtxgen/localconfig"
	"github.com/hyperledger/fabric/discovery/support/config"
	"github.com/hyperledger/fabric/discovery/support/mocks"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/onsi/gomega/gexec"
	"github.com/stretchr/testify/assert"
)

func blockWithPayload() *common.Block {
	env := &common.Envelope{
		Payload: []byte{1, 2, 3},
	}
	b, _ := proto.Marshal(env)
	return &common.Block{
		Data: &common.BlockData{
			Data: [][]byte{b},
		},
	}
}

func blockWithConfigEnvelope() *common.Block {
	pl := &common.Payload{
		Data: []byte{1, 2, 3},
	}
	plBytes, _ := proto.Marshal(pl)
	env := &common.Envelope{
		Payload: plBytes,
	}
	b, _ := proto.Marshal(env)
	return &common.Block{
		Data: &common.BlockData{
			Data: [][]byte{b},
		},
	}
}

func TestMSPIDMapping(t *testing.T) {
	randString := func() string {
		buff := make([]byte, 10)
		rand.Read(buff)
		return hex.EncodeToString(buff)
	}

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("TestMSPIDMapping_%s", randString()))
	os.Mkdir(dir, 0700)
	defer os.RemoveAll(dir)

	cryptogen, err := gexec.Build(filepath.Join("github.com", "hyperledger", "fabric", "common", "tools", "cryptogen"))
	assert.NoError(t, err)
	defer os.Remove(cryptogen)

	idemixgen, err := gexec.Build(filepath.Join("github.com", "hyperledger", "fabric", "common", "tools", "idemixgen"))
	assert.NoError(t, err)
	defer os.Remove(idemixgen)

	cryptoConfigDir := filepath.Join(dir, "crypto-config")
	b, err := exec.Command(cryptogen, "generate", fmt.Sprintf("--output=%s", cryptoConfigDir)).CombinedOutput()
	assert.NoError(t, err, string(b))

	idemixConfigDir := filepath.Join(dir, "crypto-config", "idemix")
	b, err = exec.Command(idemixgen, "ca-keygen", fmt.Sprintf("--output=%s", idemixConfigDir)).CombinedOutput()
	assert.NoError(t, err, string(b))

	profileConfig := genesisconfig.Load("TwoOrgsChannel", "testdata/")
	ordererConfig := genesisconfig.Load("TwoOrgsOrdererGenesis", "testdata/")
	profileConfig.Orderer = ordererConfig.Orderer

//Override the MSP directory with our randomly generated and populated path
	for _, org := range ordererConfig.Orderer.Organizations {
		org.MSPDir = filepath.Join(cryptoConfigDir, "ordererOrganizations", "example.com", "msp")
		org.Name = randString()
	}

//随机化组织名称
	for _, org := range profileConfig.Application.Organizations {
		org.Name = randString()
//非BCCSP MSP组织没有密码生成的加密材料，
//我们需要使用IDemix加密文件夹。
		if org.MSPType != "bccsp" {
			org.MSPDir = filepath.Join(idemixConfigDir)
			continue
		}
		org.MSPDir = filepath.Join(cryptoConfigDir, "peerOrganizations", "org1.example.com", "msp")
	}

	gen := encoder.New(profileConfig)
	block := gen.GenesisBlockForChannel("mychannel")

	fakeBlockGetter := &mocks.ConfigBlockGetter{}
	fakeBlockGetter.GetCurrConfigBlockReturnsOnCall(0, block)

	cs := config.NewDiscoverySupport(fakeBlockGetter)
	res, err := cs.Config("mychannel")

	actualKeys := make(map[string]struct{})
	for key := range res.Orderers {
		actualKeys[key] = struct{}{}
	}

	for key := range res.Msps {
		actualKeys[key] = struct{}{}
	}

//请注意，org3msp是一个idemix组织，但不应在此列出
//因为对等方不能具有IDemix凭据
	expected := map[string]struct{}{
		"OrdererMSP": {},
		"Org1MSP":    {},
		"Org2MSP":    {},
	}
	assert.Equal(t, expected, actualKeys)
}

func TestSupportGreenPath(t *testing.T) {
	fakeBlockGetter := &mocks.ConfigBlockGetter{}
	fakeBlockGetter.GetCurrConfigBlockReturnsOnCall(0, nil)

	cs := config.NewDiscoverySupport(fakeBlockGetter)
	res, err := cs.Config("test")
	assert.Nil(t, res)
	assert.Equal(t, "could not get last config block for channel test", err.Error())

	block, err := test.MakeGenesisBlock("test")
	assert.NoError(t, err)
	assert.NotNil(t, block)

	fakeBlockGetter.GetCurrConfigBlockReturnsOnCall(1, block)
	res, err = cs.Config("test")
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestSupportBadConfig(t *testing.T) {
	fakeBlockGetter := &mocks.ConfigBlockGetter{}
	cs := config.NewDiscoverySupport(fakeBlockGetter)

	fakeBlockGetter.GetCurrConfigBlockReturnsOnCall(0, &common.Block{
		Data: &common.BlockData{},
	})
	res, err := cs.Config("test")
	assert.Contains(t, err.Error(), "no transactions in block")
	assert.Nil(t, res)

	fakeBlockGetter.GetCurrConfigBlockReturnsOnCall(1, &common.Block{
		Data: &common.BlockData{
			Data: [][]byte{{1, 2, 3}},
		},
	})
	res, err = cs.Config("test")
	assert.Contains(t, err.Error(), "failed unmarshaling envelope")
	assert.Nil(t, res)

	fakeBlockGetter.GetCurrConfigBlockReturnsOnCall(2, blockWithPayload())
	res, err = cs.Config("test")
	assert.Contains(t, err.Error(), "failed unmarshaling payload")
	assert.Nil(t, res)

	fakeBlockGetter.GetCurrConfigBlockReturnsOnCall(3, blockWithConfigEnvelope())
	res, err = cs.Config("test")
	assert.Contains(t, err.Error(), "failed unmarshaling config envelope")
	assert.Nil(t, res)
}

func TestValidateConfigEnvelope(t *testing.T) {
	tests := []struct {
		name          string
		ce            *common.ConfigEnvelope
		containsError string
	}{
		{
			name:          "nil Config field",
			ce:            &common.ConfigEnvelope{},
			containsError: "field Config is nil",
		},
		{
			name: "nil ChannelGroup field",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{},
			},
			containsError: "field Config.ChannelGroup is nil",
		},
		{
			name: "nil Groups field",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{
					ChannelGroup: &common.ConfigGroup{},
				},
			},
			containsError: "field Config.ChannelGroup.Groups is nil",
		},
		{
			name: "no orderer group key",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{
					ChannelGroup: &common.ConfigGroup{
						Groups: map[string]*common.ConfigGroup{
							channelconfig.ApplicationGroupKey: {},
						},
					},
				},
			},
			containsError: "key Config.ChannelGroup.Groups[Orderer] is missing",
		},
		{
			name: "no application group key",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{
					ChannelGroup: &common.ConfigGroup{
						Groups: map[string]*common.ConfigGroup{
							channelconfig.OrdererGroupKey: {
								Groups: map[string]*common.ConfigGroup{},
							},
						},
					},
				},
			},
			containsError: "key Config.ChannelGroup.Groups[Application] is missing",
		},
		{
			name: "no groups key in orderer group",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{
					ChannelGroup: &common.ConfigGroup{
						Groups: map[string]*common.ConfigGroup{
							channelconfig.ApplicationGroupKey: {
								Groups: map[string]*common.ConfigGroup{},
							},
							channelconfig.OrdererGroupKey: {},
						},
					},
				},
			},
			containsError: "key Config.ChannelGroup.Groups[Orderer].Groups is nil",
		},
		{
			name: "no groups key in application group",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{
					ChannelGroup: &common.ConfigGroup{
						Groups: map[string]*common.ConfigGroup{
							channelconfig.ApplicationGroupKey: {},
							channelconfig.OrdererGroupKey: {
								Groups: map[string]*common.ConfigGroup{},
							},
						},
					},
				},
			},
			containsError: "key Config.ChannelGroup.Groups[Application].Groups is nil",
		},
		{
			name: "no Values in ChannelGroup",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{
					ChannelGroup: &common.ConfigGroup{
						Groups: map[string]*common.ConfigGroup{
							channelconfig.ApplicationGroupKey: {
								Groups: map[string]*common.ConfigGroup{},
							},
							channelconfig.OrdererGroupKey: {
								Groups: map[string]*common.ConfigGroup{},
							},
						},
					},
				},
			},
			containsError: "field Config.ChannelGroup.Values is nil",
		},
		{
			name: "no OrdererAddressesKey in ChannelGroup Values",
			ce: &common.ConfigEnvelope{
				Config: &common.Config{
					ChannelGroup: &common.ConfigGroup{
						Values: map[string]*common.ConfigValue{},
						Groups: map[string]*common.ConfigGroup{
							channelconfig.ApplicationGroupKey: {
								Groups: map[string]*common.ConfigGroup{},
							},
							channelconfig.OrdererGroupKey: {
								Groups: map[string]*common.ConfigGroup{},
							},
						},
					},
				},
			},
			containsError: "field Config.ChannelGroup.Values is empty",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			err := config.ValidateConfigEnvelope(test.ce)
			assert.Contains(t, test.containsError, err.Error())
		})
	}

}
