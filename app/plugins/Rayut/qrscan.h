#ifndef RAYUT_QRSCAN_H
#define RAYUT_QRSCAN_H

#ifdef __cplusplus
extern "C" {
#endif

/* Decode one QR code from an 8-bit grayscale image, one byte per pixel.
 * Returns the payload length, or -1 when no code can be read.
 * The payload is written to out and is not printed. */
int rayut_decode_gray(const unsigned char *gray, int width, int height, char *out, int out_cap);

/* 1: http(s) subscription URL. 0: other text, including a share link.
 * -1: empty. */
int rayut_route_text(const char *text);

#ifdef __cplusplus
}
#endif

#endif
