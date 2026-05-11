#pragma once

// Driver-failure probe for the QAIRT plugin.
//
// The log filter (log_filter.cpp) suppresses every QNN-originated log line so that
// end users of the CLI are not exposed to the noisy, internal QNN output. That same
// suppression also swallows the error lines that reveal *why* a model load failed
// — and the most common cause we see in the field is a too-old NPU/HTP driver.
//
// To keep the filter's "never forward raw QNN text" contract intact while still
// giving us a signal to surface, the filter flips the flag below whenever it
// suppresses a QNN-level error. Plugin code clears the flag before a load attempt
// and, if the load fails and the flag is set, emits its own clean, user-facing
// "please update your NPU driver" message.

namespace geniex::qairt {

// Reset the "saw a QNN error" latch. Call this immediately before an operation
// whose failure you want to diagnose (e.g. before `make_pipeline`).
void reset_qnn_error_flag() noexcept;

// Returns true if any QNN-originated error line has been suppressed since the
// last `reset_qnn_error_flag()`.
bool qnn_error_seen() noexcept;

// Called from the log filter when a QNN error line is suppressed. Not for
// plugin code to call directly.
void mark_qnn_error_seen() noexcept;

}  // namespace geniex::qairt
