package operator

var (
	defaultTrminationGracePeriodSeconds int64 = 5
)

func getTerminationGracePeriodSeconds() *int64 {
	return &defaultTrminationGracePeriodSeconds
}

func intToInt32(i int) *int32 {
	i32 := int32(i)
	return &i32
}
