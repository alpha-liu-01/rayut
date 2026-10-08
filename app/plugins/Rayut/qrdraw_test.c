#include "qrdraw.h"
#include "qrscan.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int round_trip(const char *text)
{
    unsigned char *pixels = NULL;
    char out[4096];
    int side = rayut_encode_qr(text, &pixels, 4);
    int length;
    if (side < 1 || !pixels) {
        fprintf(stderr, "encode failed\n");
        free(pixels);
        return 1;
    }
    memset(out, 0, sizeof out);
    length = rayut_decode_gray(pixels, side, side, out, (int)sizeof out);
    free(pixels);
    if (length < 0 || strcmp(out, text) != 0) {
        fprintf(stderr, "round trip failed\n");
        return 1;
    }
    return 0;
}

int main(void)
{
    int failed = 0;
    unsigned char *pixels = NULL;
    char long_text[5000];
    const char *share = "vless://11111111-2222-3333-4444-555555555555@example.com:443?encryption=none#lab";

    failed += round_trip(share);
    failed += round_trip("https://example.invalid/sub");
    if (rayut_encode_qr("", &pixels, 4) != -1 || pixels != NULL) {
        fprintf(stderr, "empty text drew a code\n");
        failed += 1;
    }
    free(pixels);
    pixels = NULL;
    memset(long_text, 'A', sizeof long_text - 1);
    long_text[sizeof long_text - 1] = '\0';
    if (rayut_encode_qr(long_text, &pixels, 4) != -1) {
        fprintf(stderr, "oversized text drew a code\n");
        failed += 1;
    }
    free(pixels);
    return failed == 0 ? 0 : 1;
}
