
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//Code generated by mockery v1.0.0. 不要编辑。
package mocks

import common "github.com/hyperledger/fabric/protos/common"
import mock "github.com/stretchr/testify/mock"

//BlockVerifier是BlockVerifier类型的自动生成的模拟类型
type BlockVerifier struct {
	mock.Mock
}

//verifyblocksignature提供了一个具有给定字段的模拟函数：sd，config
func (_m *BlockVerifier) VerifyBlockSignature(sd []*common.SignedData, config *common.ConfigEnvelope) error {
	ret := _m.Called(sd, config)

	var r0 error
	if rf, ok := ret.Get(0).(func([]*common.SignedData, *common.ConfigEnvelope) error); ok {
		r0 = rf(sd, config)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
