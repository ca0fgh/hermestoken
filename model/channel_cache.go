package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var channelSyncLock sync.RWMutex

type channelPriorityBucket struct {
	priority int64
	channels []*Channel
}

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
	}
	newGroup2model2channels := make(map[string]map[string][]int)

	var abilities []*Ability
	DB.Where("enabled = ?", true).Find(&abilities)
	seenAbility := make(map[string]struct{}, len(abilities))
	for _, ability := range abilities {
		if ability == nil {
			continue
		}
		channel, ok := newChannelId2channel[ability.ChannelId]
		if !ok || channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		group := strings.TrimSpace(ability.Group)
		model := strings.TrimSpace(ability.Model)
		if group == "" || model == "" {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d", group, model, ability.ChannelId)
		if _, ok := seenAbility[key]; ok {
			continue
		}
		seenAbility[key] = struct{}{}
		if _, ok := newGroup2model2channels[group]; !ok {
			newGroup2model2channels[group] = make(map[string][]int)
		}
		newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], ability.ChannelId)
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

func GetRandomSatisfiedChannel(group string, model string, retry int) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannel(group, model, retry)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := group2model2channels[group][model]

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = group2model2channels[group][normalizedModel]
	}

	if len(channels) == 0 {
		return nil, nil
	}

	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	if retry >= len(uniquePriorities) {
		retry = len(uniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetPriority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	// return null if no channel is not found
	return nil, errors.New("channel not found")
}

func GetNextSatisfiedChannel(group string, model string, priorityIndex int, excludedChannelIDs map[int]struct{}) (*Channel, int, bool, error) {
	buckets, err := getSatisfiedChannelBuckets(group, model)
	if err != nil {
		return nil, priorityIndex, false, err
	}
	if len(buckets) == 0 {
		return nil, priorityIndex, false, nil
	}

	if priorityIndex < 0 {
		priorityIndex = 0
	}
	if priorityIndex >= len(buckets) {
		return nil, priorityIndex, false, nil
	}

	for idx := priorityIndex; idx < len(buckets); idx++ {
		candidates := filterExcludedChannels(buckets[idx].channels, excludedChannelIDs)
		if len(candidates) == 0 {
			continue
		}

		channel, err := pickWeightedChannel(candidates)
		if err != nil {
			return nil, idx, false, err
		}

		hasMore := hasRemainingChannelCandidates(buckets, idx, excludedChannelIDs, channel.Id)
		return channel, idx, hasMore, nil
	}

	return nil, len(buckets), false, nil
}

func GetSatisfiedChannelPriorityIndex(group string, model string, channelID int) (int, bool, error) {
	if channelID <= 0 {
		return 0, false, nil
	}

	buckets, err := getSatisfiedChannelBuckets(group, model)
	if err != nil {
		return 0, false, err
	}
	for idx, bucket := range buckets {
		for _, channel := range bucket.channels {
			if channel.Id == channelID {
				return idx, true, nil
			}
		}
	}
	return 0, false, nil
}

func getSatisfiedChannelBuckets(group string, model string) ([]channelPriorityBucket, error) {
	var channels []*Channel
	var err error
	if common.MemoryCacheEnabled {
		channels, err = getSatisfiedChannelsFromCache(group, model)
	} else {
		channels, err = getSatisfiedChannelsFromDB(group, model)
	}
	if err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, nil
	}

	return buildChannelPriorityBuckets(channels), nil
}

func getSatisfiedChannelsFromCache(group string, model string) ([]*Channel, error) {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channelIDs := group2model2channels[group][model]
	if len(channelIDs) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channelIDs = group2model2channels[group][normalizedModel]
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}

	channels := make([]*Channel, 0, len(channelIDs))
	seen := make(map[int]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		if _, ok := seen[channelID]; ok {
			continue
		}
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		seen[channelID] = struct{}{}
		channels = append(channels, channel)
	}
	return channels, nil
}

func getSatisfiedChannelsFromDB(group string, model string) ([]*Channel, error) {
	channelIDs, err := getSatisfiedChannelIDsFromDB(group, model)
	if err != nil {
		return nil, err
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}

	var channels []*Channel
	if err := DB.Where("id in ?", channelIDs).Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

func getSatisfiedChannelIDsFromDB(group string, model string) ([]int, error) {
	channelIDs, err := loadSatisfiedChannelIDsFromDB(group, model)
	if err != nil {
		return nil, err
	}
	if len(channelIDs) > 0 {
		return channelIDs, nil
	}

	normalizedModel := ratio_setting.FormatMatchingModelName(model)
	if normalizedModel == model {
		return nil, nil
	}
	return loadSatisfiedChannelIDsFromDB(group, normalizedModel)
}

func loadSatisfiedChannelIDsFromDB(group string, model string) ([]int, error) {
	var channelIDs []int
	if err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Pluck("channel_id", &channelIDs).Error; err != nil {
		return nil, err
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}

	seen := make(map[int]struct{}, len(channelIDs))
	deduped := make([]int, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		deduped = append(deduped, channelID)
	}
	return deduped, nil
}

func buildChannelPriorityBuckets(channels []*Channel) []channelPriorityBucket {
	bucketsByPriority := make(map[int64][]*Channel)
	priorities := make([]int64, 0)
	for _, channel := range channels {
		priority := channel.GetPriority()
		if _, ok := bucketsByPriority[priority]; !ok {
			priorities = append(priorities, priority)
		}
		bucketsByPriority[priority] = append(bucketsByPriority[priority], channel)
	}

	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i] > priorities[j]
	})

	buckets := make([]channelPriorityBucket, 0, len(priorities))
	for _, priority := range priorities {
		buckets = append(buckets, channelPriorityBucket{
			priority: priority,
			channels: bucketsByPriority[priority],
		})
	}
	return buckets
}

func filterExcludedChannels(channels []*Channel, excludedChannelIDs map[int]struct{}) []*Channel {
	if len(excludedChannelIDs) == 0 {
		return channels
	}

	filtered := make([]*Channel, 0, len(channels))
	for _, channel := range channels {
		if _, ok := excludedChannelIDs[channel.Id]; ok {
			continue
		}
		filtered = append(filtered, channel)
	}
	return filtered
}

func pickWeightedChannel(channels []*Channel) (*Channel, error) {
	if len(channels) == 0 {
		return nil, errors.New("channel not found")
	}
	if len(channels) == 1 {
		return channels[0], nil
	}

	sumWeight := 0
	for _, channel := range channels {
		sumWeight += channel.GetWeight()
	}

	smoothingFactor := 1
	smoothingAdjustment := 0
	if sumWeight == 0 {
		sumWeight = len(channels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(channels) < 10 {
		smoothingFactor = 100
	}

	randomWeight := rand.Intn(sumWeight * smoothingFactor)
	for _, channel := range channels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	return nil, errors.New("channel not found")
}

func hasRemainingChannelCandidates(buckets []channelPriorityBucket, startIndex int, excludedChannelIDs map[int]struct{}, selectedChannelID int) bool {
	for idx := startIndex; idx < len(buckets); idx++ {
		for _, channel := range buckets[idx].channels {
			if channel.Id == selectedChannelID {
				continue
			}
			if _, ok := excludedChannelIDs[channel.Id]; ok {
				continue
			}
			return true
		}
	}
	return false
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	println("CacheUpdateChannel:", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)

	println("before:", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
	channelsIDM[channel.Id] = channel
	println("after :", channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex)
}
