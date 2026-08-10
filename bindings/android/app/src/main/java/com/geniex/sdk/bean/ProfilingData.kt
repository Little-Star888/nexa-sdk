package com.geniex.sdk.bean

data class ProfilingData(
    val ttftMs: Double,  /* Time to first token (ms) */
    val promptTimeMs: Double,  /* Prompt processing time (ms) */
    val decodeTimeMs: Double,   /* Token generation time (ms) */
    val promptTokens: Long,    /* Number of prompt tokens */
    val generatedTokens: Long,  /* Number of generated tokens */
    val prefillSpeed: Double,   /* Prefill speed (tokens/sec) */
    val decodingSpeed: Double,  /* Decoding speed (tokens/sec) */
    val draftNTotal: Long,       /* Speculative decoding: draft tokens generated (0 when disabled) */
    val draftNAccepted: Long,    /* Speculative decoding: draft tokens accepted by the target model */
    val stopReason: String  /* Stop reason: "eos", "length", "user", "stop_sequence" */
)
