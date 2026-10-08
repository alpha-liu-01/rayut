#include "qrdraw.h"

#include "qrcodegen.h"

#include <stdlib.h>
#include <string.h>

int rayut_encode_qr(const char *text, unsigned char **pixels, int scale)
{
    uint8_t qrcode[qrcodegen_BUFFER_LEN_MAX];
    uint8_t temp[qrcodegen_BUFFER_LEN_MAX];
    int size;
    int quiet = 4;
    int modules;
    int side;
    int y;
    unsigned char *image;

    if (!text || !pixels || scale < 1 || text[0] == '\0') {
        return -1;
    }
    if (!qrcodegen_encodeText(text, temp, qrcode, qrcodegen_Ecc_MEDIUM, qrcodegen_VERSION_MIN, qrcodegen_VERSION_MAX, qrcodegen_Mask_AUTO, true)
        && !qrcodegen_encodeText(text, temp, qrcode, qrcodegen_Ecc_LOW, qrcodegen_VERSION_MIN, qrcodegen_VERSION_MAX, qrcodegen_Mask_AUTO, true)) {
        return -1;
    }
    size = qrcodegen_getSize(qrcode);
    if (size <= 0) {
        return -1;
    }
    modules = size + quiet * 2;
    if (modules > 100000 / scale) {
        return -1;
    }
    side = modules * scale;
    image = malloc((size_t)side * (size_t)side);
    if (!image) {
        return -1;
    }
    memset(image, 255, (size_t)side * (size_t)side);
    for (y = 0; y < size; ++y) {
        int x;
        for (x = 0; x < size; ++x) {
            int dy;
            if (!qrcodegen_getModule(qrcode, x, y)) {
                continue;
            }
            for (dy = 0; dy < scale; ++dy) {
                memset(image + ((size_t)(y + quiet) * (size_t)scale + (size_t)dy) * (size_t)side + (size_t)(x + quiet) * (size_t)scale, 0, (size_t)scale);
            }
        }
    }
    *pixels = image;
    return side;
}
