package com.geniex.sdk.bean

data class VlmCreateInput(
    /**
     * Optional model identifier. The QAIRT plugin reads `metadata.json` from
     * the bundle directory directly; currently unused by
     * `geniex_resolve_device` (reserved for future model-specific defaults).
     */
    val model_name: String? = null,
    override val model_path: String,
    val mmproj_path: String? = null,
    override val config: ModelConfig,
    /**
     * [RuntimeIdValue] to use for the model
     */
    override val runtime_id: String? = null,
    /**
     * Compute unit alias. `null` selects the runtime default ([ComputeUnitValue.HYBRID]
     * for `llama_cpp`, [ComputeUnitValue.NPU] for `qairt`).
     */
    override val compute_unit: String? = null,
) : CreateInputBase
