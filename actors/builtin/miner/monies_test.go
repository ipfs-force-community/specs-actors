package miner_test

import (
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/specs-actors/v7/actors/builtin"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/v7/actors/util/smoothing"
)

// Test termination fee
func TestPledgePenaltyForTermination(t *testing.T) {
	epochTargetReward := abi.NewTokenAmount(1 << 50)
	qaSectorPower := abi.NewStoragePower(1 << 36)
	networkQAPower := abi.NewStoragePower(1 << 50)

	rewardEstimate := smoothing.TestingConstantEstimate(epochTargetReward)
	powerEstimate := smoothing.TestingConstantEstimate(networkQAPower)

	undeclaredPenalty := miner.PledgePenaltyForTerminationLowerBound(rewardEstimate, powerEstimate, qaSectorPower)
	bigInitialPledgeFactor := big.NewInt(int64(miner.InitialPledgeFactor))
	bigLifetimeCap := big.NewInt(int64(miner.TerminationLifetimeCap))

	t.Run("when undeclared fault fee exceeds expected reward, returns undeclaraed fault fee", func(t *testing.T) {
		// small pledge and means undeclared penalty will be bigger
		initialPledge := abi.NewTokenAmount(1 << 10)
		dayReward := big.Div(initialPledge, bigInitialPledgeFactor)
		twentyDayReward := big.Mul(dayReward, bigInitialPledgeFactor)
		sectorAge := 20 * abi.ChainEpoch(builtin.EpochsInDay)

		fee := miner.PledgePenaltyForTermination(dayReward, sectorAge, twentyDayReward, powerEstimate, qaSectorPower, rewardEstimate, big.Zero(), 0)

		assert.Equal(t, undeclaredPenalty, fee)
	})

	t.Run("when expected reward exceeds undeclared fault fee, returns expected reward", func(t *testing.T) {
		// initialPledge equal to undeclaredPenalty guarantees expected reward is greater
		initialPledge := undeclaredPenalty
		dayReward := big.Div(initialPledge, bigInitialPledgeFactor)
		twentyDayReward := big.Mul(dayReward, bigInitialPledgeFactor)
		sectorAgeInDays := int64(20)
		sectorAge := abi.ChainEpoch(sectorAgeInDays * builtin.EpochsInDay)

		fee := miner.PledgePenaltyForTermination(dayReward, sectorAge, twentyDayReward, powerEstimate, qaSectorPower, rewardEstimate, big.Zero(), 0)

		// expect fee to be pledge + br * age * factor where br = pledge/initialPledgeFactor
		expectedFee := big.Add(
			initialPledge,
			big.Div(
				big.Product(initialPledge, big.NewInt(sectorAgeInDays), miner.TerminationRewardFactor.Numerator),
				big.Product(bigInitialPledgeFactor, miner.TerminationRewardFactor.Denominator)))
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("sector age is capped", func(t *testing.T) {
		initialPledge := undeclaredPenalty
		dayReward := big.Div(initialPledge, bigInitialPledgeFactor)
		twentyDayReward := big.Mul(dayReward, bigInitialPledgeFactor)
		sectorAge := abi.ChainEpoch(500 * builtin.EpochsInDay)

		fee := miner.PledgePenaltyForTermination(dayReward, sectorAge, twentyDayReward, powerEstimate, qaSectorPower, rewardEstimate, big.Zero(), 0)

		// expect fee to be pledge * br * age-cap * factor where br = pledge/initialPledgeFactor
		expectedFee := big.Add(
			initialPledge,
			big.Div(
				big.Product(initialPledge, bigLifetimeCap, miner.TerminationRewardFactor.Numerator),
				big.Product(bigInitialPledgeFactor, miner.TerminationRewardFactor.Denominator)))
		assert.Equal(t, expectedFee, fee)
	})

	t.Run("fee for replacement = fee for original sector when power, BR are unchanged", func(t *testing.T) {
		// initialPledge equal to undeclaredPenalty guarantees expected reward is greater
		initialPledge := undeclaredPenalty
		dayReward := big.Div(initialPledge, bigInitialPledgeFactor)
		twentyDayReward := big.Mul(dayReward, bigInitialPledgeFactor)
		sectorAge := abi.ChainEpoch(20 * builtin.EpochsInDay)
		replacementAge := abi.ChainEpoch(2 * builtin.EpochsInDay)

		// use low power, so we don't test SP=SP
		power := big.NewInt(1)

		// fee for old sector if had terminated when it was replaced
		unreplacedFee := miner.PledgePenaltyForTermination(dayReward, sectorAge, twentyDayReward, powerEstimate, power, rewardEstimate, big.Zero(), 0)

		// actual fee including replacement parameters
		actualFee := miner.PledgePenaltyForTermination(dayReward, replacementAge, twentyDayReward, powerEstimate, power, rewardEstimate, dayReward, sectorAge-replacementAge)

		assert.Equal(t, unreplacedFee, actualFee)
	})

	t.Run("fee for replacement = fee for same sector without replacement after lifetime cap", func(t *testing.T) {
		// initialPledge equal to undeclaredPenalty guarantees expected reward is greater
		initialPledge := undeclaredPenalty
		dayReward := big.Div(initialPledge, bigInitialPledgeFactor)
		twentyDayReward := big.Mul(dayReward, bigInitialPledgeFactor)
		sectorAge := abi.ChainEpoch(20 * builtin.EpochsInDay)
		replacementAge := abi.ChainEpoch(miner.TerminationLifetimeCap+1) * builtin.EpochsInDay

		// use low power, so we don't test SP=SP
		power := big.NewInt(1)

		// fee for new sector with no replacement
		noReplace := miner.PledgePenaltyForTermination(dayReward, replacementAge, twentyDayReward, powerEstimate, power, rewardEstimate, big.Zero(), 0)

		// actual fee including replacement parameters
		withReplace := miner.PledgePenaltyForTermination(dayReward, replacementAge, twentyDayReward, powerEstimate, power, rewardEstimate, dayReward, sectorAge)

		assert.Equal(t, noReplace, withReplace)
	})

	t.Run("charges for replaced sector at replaced sector day rate", func(t *testing.T) {
		// initialPledge equal to undeclaredPenalty guarantees expected reward is greater
		initialPledge := undeclaredPenalty
		dayReward := big.Div(initialPledge, bigInitialPledgeFactor)
		oldDayReward := big.Mul(big.NewInt(2), dayReward)
		twentyDayReward := big.Mul(dayReward, bigInitialPledgeFactor)
		oldSectorAgeInDays := int64(20)
		oldSectorAge := abi.ChainEpoch(oldSectorAgeInDays * builtin.EpochsInDay)
		replacementAgeInDays := int64(15)
		replacementAge := abi.ChainEpoch(replacementAgeInDays * builtin.EpochsInDay)

		// use low power, so termination fee exceeds SP
		power := big.NewInt(1)

		oldPenalty := big.Div(
			big.Product(oldDayReward, big.NewInt(oldSectorAgeInDays), miner.TerminationRewardFactor.Numerator),
			miner.TerminationRewardFactor.Denominator,
		)
		newPenalty := big.Div(
			big.Product(dayReward, big.NewInt(replacementAgeInDays), miner.TerminationRewardFactor.Numerator),
			miner.TerminationRewardFactor.Denominator,
		)
		expectedFee := big.Sum(twentyDayReward, oldPenalty, newPenalty)

		fee := miner.PledgePenaltyForTermination(dayReward, replacementAge, twentyDayReward, powerEstimate, power, rewardEstimate, oldDayReward, oldSectorAge)

		assert.Equal(t, expectedFee, fee)
	})
}

func TestNegativeBRClamp(t *testing.T) {
	epochTargetReward := abi.NewTokenAmount(1 << 50)
	qaSectorPower := abi.NewStoragePower(1 << 36)
	networkQAPower := abi.NewStoragePower(1 << 10)
	powerRateOfChange := abi.NewStoragePower(1 << 10).Neg()
	rewardEstimate := smoothing.NewEstimate(epochTargetReward, big.Zero())
	powerEstimate := smoothing.NewEstimate(networkQAPower, powerRateOfChange)

	fourBR := miner.ExpectedRewardForPower(rewardEstimate, powerEstimate, qaSectorPower, abi.ChainEpoch(4))
	assert.Equal(t, big.Zero(), fourBR)
}

func TestContinuedFault(t *testing.T) {
	t.Run("zero power means zero fault penalty", func(t *testing.T) {
		epochTargetReward := abi.NewTokenAmount(1 << 50)
		zeroQAPower := abi.NewStoragePower(0)
		networkQAPower := abi.NewStoragePower(1 << 10)
		powerRateOfChange := abi.NewStoragePower(1 << 10)
		rewardEstimate := smoothing.NewEstimate(epochTargetReward, big.Zero())
		powerEstimate := smoothing.NewEstimate(networkQAPower, powerRateOfChange)

		penaltyForZeroPowerFaulted := miner.PledgePenaltyForContinuedFault(rewardEstimate, powerEstimate, zeroQAPower)
		assert.Equal(t, big.Zero(), penaltyForZeroPowerFaulted)
	})
}

func TestExpectedRewardForPowerClamptedAtAttoFIL(t *testing.T) {
	t.Run("expected zero valued BR clamped at 1 attofil", func(t *testing.T) {
		epochTargetReward := abi.NewTokenAmount(1 << 50)
		zeroQAPower := abi.NewStoragePower(0)
		networkQAPower := abi.NewStoragePower(1 << 10)
		powerRateOfChange := abi.NewStoragePower(1 << 10)
		rewardEstimate := smoothing.NewEstimate(epochTargetReward, big.Zero())
		powerEstimate := smoothing.NewEstimate(networkQAPower, powerRateOfChange)

		brClamped := miner.ExpectedRewardForPowerClampedAtAttoFIL(rewardEstimate, powerEstimate, zeroQAPower, abi.ChainEpoch(1))
		assert.Equal(t, big.NewInt(1), brClamped)
	})

	t.Run("expected negative valued BR clamped at 1 atto FIL", func(t *testing.T) {
		epochTargetReward := abi.NewTokenAmount(1 << 50)
		qaSectorPower := abi.NewStoragePower(1 << 36)
		networkQAPower := abi.NewStoragePower(1 << 10)
		powerRateOfChange := abi.NewStoragePower(1 << 10).Neg()
		rewardEstimate := smoothing.NewEstimate(epochTargetReward, big.Zero())
		powerEstimate := smoothing.NewEstimate(networkQAPower, powerRateOfChange)

		fourBRClamped := miner.ExpectedRewardForPowerClampedAtAttoFIL(rewardEstimate, powerEstimate, qaSectorPower, abi.ChainEpoch(4))
		assert.Equal(t, big.NewInt(1), fourBRClamped)
	})

}

func TestPrecommitDepositAndInitialPledgePostiive(t *testing.T) {
	epochTargetReward := abi.NewTokenAmount(0) // zero reward so IP Base unclamped is 0
	qaSectorPower := abi.NewStoragePower(1 << 36)
	networkQAPower := abi.NewStoragePower(1 << 10)
	baselinePower := networkQAPower
	powerRateOfChange := abi.NewStoragePower(1 << 10)
	rewardEstimate := smoothing.NewEstimate(epochTargetReward, big.Zero())
	powerEstimate := smoothing.NewEstimate(networkQAPower, powerRateOfChange)
	circulatingSupply := abi.NewTokenAmount(0)
	t.Run("IP is clamped at 1 attofil", func(t *testing.T) {
		ip := miner.InitialPledgeForPower(qaSectorPower, baselinePower, rewardEstimate, powerEstimate, circulatingSupply)
		assert.Equal(t, abi.NewTokenAmount(1), ip)
	})
	t.Run("PCD is clamped at 1 attoFIL", func(t *testing.T) {
		pcd := miner.PreCommitDepositForPower(rewardEstimate, powerEstimate, qaSectorPower)
		assert.Equal(t, abi.NewTokenAmount(1), pcd)
	})
}

func TestAggregateNetworkFee(t *testing.T) {

	t.Run("Constant fee per sector when base fee is below 5 nFIL", func(t *testing.T) {
		feeFuncs := []func(int, abi.TokenAmount) abi.TokenAmount{miner.AggregateProveCommitNetworkFee, miner.AggregatePreCommitNetworkFee}
		for _, feeFunc := range feeFuncs {

			oneSectorFee := feeFunc(1, big.Zero())
			tenSectorFee := feeFunc(10, big.Zero())
			assert.Equal(t, big.Mul(oneSectorFee, big.NewInt(10)), tenSectorFee)
			fortySectorFee := feeFunc(40, builtin.OneNanoFIL)
			assert.Equal(t, big.Mul(oneSectorFee, big.NewInt(40)), fortySectorFee)
			twoHundredSectorFee := feeFunc(200, big.Mul(big.NewInt(3), builtin.OneNanoFIL))
			assert.Equal(t, big.Mul(oneSectorFee, big.NewInt(200)), twoHundredSectorFee)
		}
	})

	t.Run("Fee increases iff basefee crosses threshold", func(t *testing.T) {
		feeFuncs := []func(int, abi.TokenAmount) abi.TokenAmount{miner.AggregateProveCommitNetworkFee, miner.AggregatePreCommitNetworkFee}
		for _, feeFunc := range feeFuncs {

			atNoBaseFee := feeFunc(10, big.Zero())
			atBalanceMinusOneBaseFee := feeFunc(10, big.Sub(miner.BatchBalancer, builtin.OneNanoFIL))
			atBalanceBaseFee := feeFunc(10, miner.BatchBalancer)
			atBalancePlusOneBaseFee := feeFunc(10, big.Sum(miner.BatchBalancer, builtin.OneNanoFIL))
			atBalancePlusTwoBaseFee := feeFunc(10, big.Sum(miner.BatchBalancer, builtin.OneNanoFIL, builtin.OneNanoFIL))
			atBalanceTimesTwoBaseFee := feeFunc(10, big.Mul(miner.BatchBalancer, big.NewInt(2)))

			assert.True(t, atNoBaseFee.Equals(atBalanceMinusOneBaseFee))
			assert.True(t, atNoBaseFee.Equals(atBalanceBaseFee))
			assert.True(t, atBalanceBaseFee.LessThan(atBalancePlusOneBaseFee))
			assert.True(t, atBalancePlusOneBaseFee.LessThan(atBalancePlusTwoBaseFee))
			assert.True(t, atBalanceTimesTwoBaseFee.Equals(big.Mul(big.NewInt(2), atBalanceBaseFee)))
		}
	})

	t.Run("Regression tests", func(t *testing.T) {
		tenAtNoBaseFee := big.Sum(miner.AggregateProveCommitNetworkFee(10, big.Zero()), miner.AggregatePreCommitNetworkFee(10, big.Zero()))
		assert.Equal(t, big.Div(big.Product(builtin.OneNanoFIL, big.NewInt(5), big.NewInt(65733297)), big.NewInt(2)), tenAtNoBaseFee) // (5/20) * x * 10 = (5/2) * x

		tenAtOneNanoBaseFee := big.Sum(miner.AggregateProveCommitNetworkFee(10, builtin.OneNanoFIL), miner.AggregatePreCommitNetworkFee(10, builtin.OneNanoFIL))
		assert.Equal(t, big.Div(big.Product(builtin.OneNanoFIL, big.NewInt(5), big.NewInt(65733297)), big.NewInt(2)), tenAtOneNanoBaseFee) // (5/20) * x * 10 = (5/2) * x

		hundredAtThreeNanoBaseFee := big.Sum(miner.AggregateProveCommitNetworkFee(100, big.Mul(big.NewInt(3), builtin.OneNanoFIL)),
			miner.AggregatePreCommitNetworkFee(100, big.Mul(big.NewInt(3), builtin.OneNanoFIL)))
		assert.Equal(t, big.Div(big.Product(builtin.OneNanoFIL, big.NewInt(50), big.NewInt(65733297)), big.NewInt(2)), hundredAtThreeNanoBaseFee)

		hundredAtSixNanoBaseFee := big.Sum(miner.AggregateProveCommitNetworkFee(100, big.Mul(big.NewInt(6), builtin.OneNanoFIL)),
			miner.AggregatePreCommitNetworkFee(100, big.Mul(big.NewInt(6), builtin.OneNanoFIL)))
		assert.Equal(t, big.Product(builtin.OneNanoFIL, big.NewInt(30), big.NewInt(65733297)), hundredAtSixNanoBaseFee)
	})

	t.Run("25/75 split", func(t *testing.T) {
		// check 25/75% split up to uFIL precision
		oneMicroFIL := big.Mul(builtin.OneNanoFIL, big.NewInt(1000))
		atNoBaseFeePre := big.Div(miner.AggregatePreCommitNetworkFee(13, big.Zero()), oneMicroFIL)
		atNoBaseFeeProve := big.Div(miner.AggregateProveCommitNetworkFee(13, big.Zero()), oneMicroFIL)
		assert.Equal(t, atNoBaseFeeProve, big.Mul(big.NewInt(3), atNoBaseFeePre))

		atFiveBaseFeePre := big.Div(miner.AggregatePreCommitNetworkFee(303, big.Mul(big.NewInt(5), builtin.OneNanoFIL)), oneMicroFIL)
		atFiveBaseFeeProve := big.Div(miner.AggregateProveCommitNetworkFee(303, big.Mul(big.NewInt(5), builtin.OneNanoFIL)), oneMicroFIL)
		assert.Equal(t, atFiveBaseFeeProve, big.Mul(big.NewInt(3), atFiveBaseFeePre))

		atTwentyBaseFeePre := big.Div(miner.AggregatePreCommitNetworkFee(13, big.Mul(big.NewInt(20), builtin.OneNanoFIL)), oneMicroFIL)
		atTwentyBaseFeeProve := big.Div(miner.AggregateProveCommitNetworkFee(13, big.Mul(big.NewInt(20), builtin.OneNanoFIL)), oneMicroFIL)
		assert.Equal(t, atTwentyBaseFeeProve, big.Mul(big.NewInt(3), atTwentyBaseFeePre))
	})
}
