package strutil

// StringPtr returns a pointer to the given string value.
func StringPtr(s string) *string { return &s }

// Int32Ptr returns a pointer to the given int32 value.
func Int32Ptr(v int32) *int32 { return &v }