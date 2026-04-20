/**
 * End-to-end test for the model manager C API.
 *
 * Build (after cmake -DGENIEX_MODEL_MANAGER=ON):
 *
 *   cc test_model_manager.c \
 *       -I../include -I../../pkg-geniex/include \
 *       -L../../pkg-geniex/lib -lgeniex \
 *       -Wl,-rpath,../../pkg-geniex/lib \
 *       -o test_model_manager
 *
 * Run:
 *   GENIEX_DATADIR=/tmp/geniex-test ./test_model_manager
 *
 * The test downloads a tiny model (smolvlm, ~500 MB). Set HF_TOKEN env var
 * if you have a HuggingFace token to avoid rate limiting.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "ml.h"
#include "ml_model.h"

#define CHECK(call)                                                  \
    do {                                                             \
        int32_t _rc = (call);                                        \
        if (_rc != ML_SUCCESS) {                                     \
            fprintf(stderr, "FAIL  %s  (code=%d)\n", #call, _rc);   \
            return 1;                                                 \
        }                                                            \
        printf("OK    %s\n", #call);                                 \
    } while (0)

static bool progress_cb(int64_t downloaded, int64_t total, void* ud) {
    (void)ud;
    printf("      progress: %lld / %lld\n", (long long)downloaded, (long long)total);
    return true;
}

int main(void) {
    const char* data_dir = getenv("GENIEX_DATADIR");
    const char* hf_token = getenv("HF_TOKEN");

    printf("=== ml_model_init ===\n");
    CHECK(ml_model_init(data_dir, hf_token));

    /* ---- resolve alias ---- */
    printf("\n=== ml_model_resolve_alias ===\n");
    char* full_name = NULL;
    CHECK(ml_model_resolve_alias("smolvlm", &full_name));
    printf("      smolvlm -> %s\n", full_name);
    ml_free(full_name);

    /* ---- pull ---- */
    printf("\n=== ml_model_pull ===\n");
    ml_ModelPullInput pull_input = {
        .model_name  = "ggml-org/SmolVLM-500M-Instruct-GGUF",
        .quant       = NULL,
        .hub         = ML_HUB_HUGGINGFACE,
        .local_path  = NULL,
        .on_progress = progress_cb,
        .user_data   = NULL,
    };
    CHECK(ml_model_pull(&pull_input));

    /* ---- list ---- */
    printf("\n=== ml_model_list ===\n");
    ml_ModelListOutput list_out = {0};
    CHECK(ml_model_list(&list_out));
    printf("      cached models (%d):\n", list_out.count);
    for (int i = 0; i < list_out.count; i++) {
        printf("        [%d] %s\n", i, list_out.names[i]);
    }
    ml_model_list_free(&list_out);

    /* ---- get_type ---- */
    printf("\n=== ml_model_get_type ===\n");
    ml_ModelType mtype;
    CHECK(ml_model_get_type("ggml-org/SmolVLM-500M-Instruct-GGUF", &mtype));
    printf("      model type: %d (expected %d for vlm)\n", mtype, ML_MODEL_TYPE_VLM);

    /* ---- get_paths ---- */
    printf("\n=== ml_model_get_paths ===\n");
    ml_ModelPaths paths = {0};
    CHECK(ml_model_get_paths("ggml-org/SmolVLM-500M-Instruct-GGUF", &paths));
    printf("      model_path:     %s\n", paths.model_path     ? paths.model_path     : "(null)");
    printf("      mmproj_path:    %s\n", paths.mmproj_path    ? paths.mmproj_path    : "(null)");
    printf("      tokenizer_path: %s\n", paths.tokenizer_path ? paths.tokenizer_path : "(null)");
    printf("      model_dir:      %s\n", paths.model_dir      ? paths.model_dir      : "(null)");
    printf("      model_name:     %s\n", paths.model_name     ? paths.model_name     : "(null)");
    printf("      plugin_id:      %s\n", paths.plugin_id      ? paths.plugin_id      : "(null)");
    ml_model_paths_free(&paths);

    /* ---- remove ---- */
    printf("\n=== ml_model_remove ===\n");
    CHECK(ml_model_remove("ggml-org/SmolVLM-500M-Instruct-GGUF"));

    /* ---- verify list is empty ---- */
    printf("\n=== ml_model_list (after remove) ===\n");
    CHECK(ml_model_list(&list_out));
    printf("      cached models after remove: %d (expected 0)\n", list_out.count);
    ml_model_list_free(&list_out);

    CHECK(ml_model_deinit());

    printf("\n=== ALL TESTS PASSED ===\n");
    return 0;
}
