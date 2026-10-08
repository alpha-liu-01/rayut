#ifndef RAYUT_QRDRAW_H
#define RAYUT_QRDRAW_H

#ifdef __cplusplus
extern "C" {
#endif

/* Draw one QR code as 8-bit grayscale, 0 for dark and 255 for light.
 * scale is pixels per module, including a four-module quiet zone.
 * Returns the side length in pixels, or -1 when the text cannot be drawn.
 * The caller frees *pixels. The text is not printed. */
int rayut_encode_qr(const char *text, unsigned char **pixels, int scale);

#ifdef __cplusplus
}
#endif

#endif
