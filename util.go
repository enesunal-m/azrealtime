package azrealtime

// Ptr is a utility function that returns a pointer to the given value.
// This is useful for setting optional fields in structs that require pointers,
// such as Session configuration fields.
//
// Example usage:
//   session := Session{
//       Voice: Ptr("alloy"),
//       Instructions: Ptr("You are a helpful assistant."),
//   }
func Ptr[T any](v T) *T { return &v }
