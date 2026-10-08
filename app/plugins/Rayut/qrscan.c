#include "qrscan.h"

#include "quirc.h"

#include <stddef.h>
#include <stdint.h>
#include <string.h>

int rayut_decode_gray(const unsigned char *gray, int width, int height, char *out, int out_cap)
{
    struct quirc *qr;
    int count;
    int index;
    uint8_t *buffer;
    int buffer_width = 0;
    int buffer_height = 0;

    if (!gray || !out || width <= 0 || height <= 0 || out_cap <= 0) {
        return -1;
    }
    qr = quirc_new();
    if (!qr || quirc_resize(qr, width, height) < 0) {
        quirc_destroy(qr);
        return -1;
    }
    buffer = quirc_begin(qr, &buffer_width, &buffer_height);
    if (!buffer || buffer_width != width || buffer_height != height) {
        quirc_destroy(qr);
        return -1;
    }
    memcpy(buffer, gray, (size_t)width * (size_t)height);
    quirc_end(qr);
    count = quirc_count(qr);
    for (index = 0; index < count; ++index) {
        struct quirc_code code;
        struct quirc_data data;
        quirc_extract(qr, index, &code);
        if (quirc_decode(&code, &data) != QUIRC_SUCCESS || data.payload_len <= 0) {
            continue;
        }
        if (data.payload_len >= out_cap) {
            quirc_destroy(qr);
            return -1;
        }
        memcpy(out, data.payload, (size_t)data.payload_len);
        out[data.payload_len] = '\0';
        quirc_destroy(qr);
        return data.payload_len;
    }
    quirc_destroy(qr);
    return -1;
}

int rayut_route_text(const char *text)
{
    const char *end;
    if (!text) {
        return -1;
    }
    while (*text == ' ' || *text == '\t' || *text == '\n' || *text == '\r') {
        ++text;
    }
    if (*text == '\0') {
        return -1;
    }
    end = text + strlen(text);
    while (end > text && (end[-1] == ' ' || end[-1] == '\t' || end[-1] == '\n' || end[-1] == '\r')) {
        --end;
    }
    if ((size_t)(end - text) >= 8 && strncmp(text, "https://", 8) == 0) {
        text += 8;
    } else if ((size_t)(end - text) >= 7 && strncmp(text, "http://", 7) == 0) {
        text += 7;
    } else {
        return 0;
    }
    if (text >= end) {
        return 0;
    }
    for (; text < end; ++text) {
        if (*text == ' ' || *text == '\n' || *text == '\r' || *text == '\t') {
            return 0;
        }
    }
    return 1;
}
