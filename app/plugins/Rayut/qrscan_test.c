#include "qrscan.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static unsigned char *read_pgm(const char *path, int *width, int *height)
{
    FILE *file = fopen(path, "rb");
    char magic[8];
    int max_value = 0;
    unsigned char *pixels;
    size_t count;
    if (!file) {
        return NULL;
    }
    if (fscanf(file, "%7s", magic) != 1 || strcmp(magic, "P5") != 0) {
        fclose(file);
        return NULL;
    }
    if (fscanf(file, "%d %d %d", width, height, &max_value) != 3 || *width <= 0 || *height <= 0 || max_value > 255) {
        fclose(file);
        return NULL;
    }
    if (fgetc(file) == EOF) {
        fclose(file);
        return NULL;
    }
    count = (size_t)(*width) * (size_t)(*height);
    pixels = malloc(count);
    if (!pixels || fread(pixels, 1, count, file) != count) {
        free(pixels);
        fclose(file);
        return NULL;
    }
    fclose(file);
    return pixels;
}

static int expect_text(const char *path, const char *want)
{
    int width = 0;
    int height = 0;
    unsigned char *pixels = read_pgm(path, &width, &height);
    char out[4096];
    int length;
    if (!pixels) {
        fprintf(stderr, "unreadable %s\n", path);
        return 1;
    }
    memset(out, 0, sizeof out);
    length = rayut_decode_gray(pixels, width, height, out, (int)sizeof out);
    free(pixels);
    if (length < 0 || strcmp(out, want) != 0) {
        fprintf(stderr, "decode mismatch for %s\n", path);
        return 1;
    }
    return 0;
}

int main(void)
{
    int failed = 0;
    int width = 0;
    int height = 0;
    unsigned char *blank;
    char out[64];
    const char *share = "vless://11111111-2222-3333-4444-555555555555@example.com:443?encryption=none#lab";
    const char *sub = "https://example.invalid/sub";

    failed += expect_text("testdata/share.pgm", share);
    failed += expect_text("testdata/sub.pgm", sub);
    if (rayut_route_text(share) != 0 || rayut_route_text(sub) != 1 || rayut_route_text("  ") != -1) {
        fprintf(stderr, "route mismatch\n");
        failed += 1;
    }
    blank = read_pgm("testdata/blank.pgm", &width, &height);
    memset(out, 0, sizeof out);
    if (!blank || rayut_decode_gray(blank, width, height, out, (int)sizeof out) != -1 || out[0] != '\0') {
        fprintf(stderr, "blank image was accepted\n");
        failed += 1;
    }
    free(blank);
    return failed == 0 ? 0 : 1;
}
