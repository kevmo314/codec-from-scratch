package main

import (
	"bytes"
	"compress/flate"
	"flag"
	"io"
	"log"
	"os"
)

// This script shows how to build a basic video encoder. In the real world, video encoders
// are a lot more complex than this, achieving upwards of 99.9% compression or more, but
// this guide will show how we can achieve 90% compression with a simple encoder.
//
// Fundamentally, video encoding is much like image encoding but with the ability to compress
// temporally. Image compression often takes advantage of the human eye's insensitivity to
// small changes in color, which we will also take advantage of in this encoder.
//
// Additionally, we will stick to older techniques and skip over more modern ones that involve
// a lot more math. This is to focus on the core concepts of video encoding instead of
// getting lost in the "optimal" encoding approach.
//
// Run this code with:
//   cat video.rgb24 | go run main.go

func main() {
	var width, height int
	flag.IntVar(&width, "width", 384, "width of the video")
	flag.IntVar(&height, "height", 216, "height of the video")
	flag.Parse()

	frames := make([][]byte, 0)

	for {
		// Read raw video frames from stdin. In rgb24 format, each pixel (r, g, b) is one byte
		// so the total size of the frame is width * height * 3.

		frame := make([]byte, width*height*3)

		// read the frame from stdin
		if _, err := io.ReadFull(os.Stdin, frame); err != nil {
			break
		}

		frames = append(frames, frame)
	}

	// Now we have our raw video, using a truly ridiculous amount of memory!

	rawSize := size(frames)
	log.Printf("Raw size: %d bytes", rawSize)

	for i, frame := range frames {
		// First, we will convert each frame to YUV420 format. Each pixel in RGB24 format
		// looks like this:
		//
		// +-----------+-----------+-----------+-----------+
		// |           |           |           |           |
		// | (r, g, b) | (r, g, b) | (r, g, b) | (r, g, b) |
		// |           |           |           |           |
		// +-----------+-----------+-----------+-----------+
		// |           |           |           |           |
		// | (r, g, b) | (r, g, b) | (r, g, b) | (r, g, b) |
		// |           |           |           |           |
		// +-----------+-----------+-----------+-----------+  ...
		// |           |           |           |           |
		// | (r, g, b) | (r, g, b) | (r, g, b) | (r, g, b) |
		// |           |           |           |           |
		// +-----------+-----------+-----------+-----------+
		// |           |           |           |           |
		// | (r, g, b) | (r, g, b) | (r, g, b) | (r, g, b) |
		// |           |           |           |           |
		// +-----------+-----------+-----------+-----------+
		//
		//                        ...
		//
		// YUV420 format looks like this:
		//
		// +-----------+-----------+-----------+-----------+
		// |  Y(0, 0)  |  Y(0, 1)  |  Y(0, 2)  |  Y(0, 3)  |
		// |  U(0, 0)  |  U(0, 0)  |  U(0, 1)  |  U(0, 1)  |
		// |  V(0, 0)  |  V(0, 0)  |  V(0, 1)  |  V(0, 1)  |
		// +-----------+-----------+-----------+-----------+
		// |  Y(1, 0)  |  Y(1, 1)  |  Y(1, 2)  |  Y(1, 3)  |
		// |  U(0, 0)  |  U(0, 0)  |  U(0, 1)  |  U(0, 1)  |
		// |  V(0, 0)  |  V(0, 0)  |  V(0, 1)  |  V(0, 1)  |
		// +-----------+-----------+-----------+-----------+  ...
		// |  Y(2, 0)  |  Y(2, 1)  |  Y(2, 2)  |  Y(2, 3)  |
		// |  U(1, 0)  |  U(1, 0)  |  U(1, 1)  |  U(1, 1)  |
		// |  V(1, 0)  |  V(1, 0)  |  V(1, 1)  |  V(1, 1)  |
		// +-----------+-----------+-----------+-----------+
		// |  Y(3, 0)  |  Y(3, 1)  |  Y(3, 2)  |  Y(3, 3)  |
		// |  U(1, 0)  |  U(1, 0)  |  U(1, 1)  |  U(1, 1)  |
		// |  V(1, 0)  |  V(1, 0)  |  V(1, 1)  |  V(1, 1)  |
		// +-----------+-----------+-----------+-----------+
		//					      ...
		//
		// The gist of this format is that instead of the components R, G, B which each
		// pixel needs, we first convert it to a different space, Y (luminance) and UV (chrominance).
		// The way to think about this is that the Y component is the brightness of the pixel,
		// and the UV components are the color of the pixel. The UV components are shared
		// between 4 adjacent pixels, so we only need to store them once for each 4 pixels.
		//
		// The intuition is that the human eye is more sensitive to brightness than color,
		// so we can store the brightness of each pixel and then store the color of each
		// 4 pixels. This is a huge space savings, since we only need to store 1/4 of the
		// pixels in the image.
		//
		// If you're seeking more resources, YUV format is also known as YCbCr.
		// Actually that's not completely true, but it's close enough and color space selection
		// is a whole other topic.
		//
		// By convention, in our byte slice, we store reading left to right then top to bottom.
		// That is, to find a pixel at row i, column j, we would find the byte at index
		// (i * width + j) * 3.
		//
		// In practice, this doesn't matter that much because our image will be transposed if
		// this is done backwards. The important thing is that we are consistent.

		Y := make([]byte, width*height)
		U := make([]float64, width*height)
		V := make([]float64, width*height)
		for j := 0; j < width*height; j++ {
			// Convert the pixel from RGB to YUV
			r, g, b := float64(frame[3*j]), float64(frame[3*j+1]), float64(frame[3*j+2])

			// These coefficients are from the ITU-R standard.
			// See https://en.wikipedia.org/wiki/YUV#Y%E2%80%B2UV444_to_RGB888_conversion
			//
			// In practice, the actual coefficients vary based on the standard.
			// For our example, it doesn't matter that much, the key insight is
			// more that converting to YUV allows us to downsample the color
			// space efficiently.
			y := +0.299*r + 0.587*g + 0.114*b
			u := -0.169*r - 0.331*g + 0.449*b + 128
			v := 0.499*r - 0.418*g - 0.0813*b + 128

			// Store the YUV values in our byte slices. These are separated to make the
			// next step a bit easier.
			Y[j] = uint8(y)
			U[j] = u
			V[j] = v
		}

		// Now, we will downsample the U and V components. This is a process where we
		// take the 4 pixels that share a U and V component and average them together.

		// We will store the downsampled U and V components in these slices.
		uDownsampled := make([]byte, width*height/4)
		vDownsampled := make([]byte, width*height/4)
		for x := 0; x < height; x += 2 {
			for y := 0; y < width; y += 2 {
				// We will average the U and V components of the 4 pixels that share this
				// U and V component.
				u := (U[x*width+y] + U[x*width+y+1] + U[(x+1)*width+y] + U[(x+1)*width+y+1]) / 4
				v := (V[x*width+y] + V[x*width+y+1] + V[(x+1)*width+y] + V[(x+1)*width+y+1]) / 4

				// Store the downsampled U and V components in our byte slices.
				uDownsampled[x/2*width/2+y/2] = uint8(u)
				vDownsampled[x/2*width/2+y/2] = uint8(v)
			}
		}

		yuvFrame := make([]byte, len(Y)+len(uDownsampled)+len(vDownsampled))

		// Now we need to store the YUV values in a byte slice. To make the data more
		// compressible, we will store all the Y values first, then all the U values,
		// then all the V values. This is called a planar format.
		//
		// The intuition is that adjacent Y, U, and V values are more likely to be
		// similar than Y, U, and V themselves. Therefore, storing the components
		// in a planar format will save more data later.

		copy(yuvFrame, Y)
		copy(yuvFrame[len(Y):], uDownsampled)
		copy(yuvFrame[len(Y)+len(uDownsampled):], vDownsampled)

		frames[i] = yuvFrame
	}

	// Now we have our YUV-encoded video, which takes half the space!

	yuvSize := size(frames)
	log.Printf("YUV420P size: %d bytes (%0.2f%% original size)", yuvSize, 100*float32(yuvSize)/float32(rawSize))

	// We can also write this out to a file, which can be played with ffplay:
	//
	//   ffplay -f rawvideo -pixel_format yuv420p -video_size 384x216 -framerate 25 encoded.yuv

	if err := os.WriteFile("encoded.yuv", bytes.Join(frames, nil), 0644); err != nil {
		log.Fatal(err)
	}

	encoded := make([][]byte, len(frames))
	for i := range frames {
		// Next, we will simplify the data by computing the delta between each frame.
		// Observe that in many cases, pixels between frames don't change much. Therefore,
		// many of the deltas will be small. We can store these small deltas more efficiently.
		//
		// Of course, the first frame doesn't have a previous frame so we will store the entire thing.
		// This is called a keyframe. In the real world, keyframes are computed periodically and
		// demarcated in the metadata. Keyframes can also be compressed, but we will deal with that later.
		// In our encoder, we will (by convention) make frame 0 the keyframe.
		//
		// The rest of the frames will delta from the previous frame. These are called predicted frames,
		// also known as P-frames.

		if i == 0 {
			// This is the keyframe, store the raw frame.
			encoded[i] = frames[i]
			continue
		}

		delta := make([]byte, len(frames[i]))
		for j := 0; j < len(delta); j++ {
			delta[j] = frames[i][j] - frames[i-1][j]
		}

		// Now we have our delta frame, which if we print out contains a bunch of zeroes (woah!).
		// These zeros are pretty compressible, so we will compress them with run length encoding.
		// This is a simple algorithm where we store the number of times a value repeats, then the value.
		//
		// For example, the sequence 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0
		// would be stored as 4, 0, 12, 1, 4, 0.
		//
		// Run length encoding is no longer used in modern codecs, but it's a good exercise and sufficient
		// to achieve our compression goals.

		var rle []byte
		for j := 0; j < len(delta); {
			// Count the number of times the current value repeats.
			var count byte
			for count = 0; count < 255 && j+int(count) < len(delta) && delta[j+int(count)] == delta[j]; count++ {
			}

			// Store the count and value.
			rle = append(rle, count)
			rle = append(rle, delta[j])

			j += int(count)
		}

		// Save the RLE frame.
		encoded[i] = rle
	}

	rleSize := size(encoded)
	log.Printf("RLE size: %d bytes (%0.2f%% original size)", rleSize, 100*float32(rleSize)/float32(rawSize))

	// This is good, we're at 1/4 the size of the original video. But we can do better.
	// Note that most of our longest runs are runs of zeros. This is because the delta
	// between frames is usually small. We have a bit of flexibility in choice of algorithm
	// here, so to keep the encoder simple, we will defer to using the DEFLATE algorithm
	// which is available in the standard library. The implementation is beyond the scope
	// of this demonstration.

	var deflated bytes.Buffer
	w, err := flate.NewWriter(&deflated, flate.BestCompression)
	if err != nil {
		log.Fatal(err)
	}
	for i := range frames {
		if i == 0 {
			// This is the keyframe, write the raw frame.
			if _, err := w.Write(frames[i]); err != nil {
				log.Fatal(err)
			}
			continue
		}

		delta := make([]byte, len(frames[i]))
		for j := 0; j < len(delta); j++ {
			delta[j] = frames[i][j] - frames[i-1][j]
		}
		if _, err := w.Write(delta); err != nil {
			log.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		log.Fatal(err)
	}

	deflatedSize := deflated.Len()
	log.Printf("DEFLATE size: %d bytes (%0.2f%% original size)", deflatedSize, 100*float32(deflatedSize)/float32(rawSize))

	// You'll note that the DEFLATE step takes quite a while to run. In general, encoders tend to run
	// much slower than decoders. This is true for most compression algorithms, not just video codecs.
	// This is because the encoder needs to do a lot of work to analyze the data and make decisions
	// about how to compress it. The decoder, on the other hand, is just a simple loop that reads the
	// data and does the opposite of the encoder.
	//
	// At this point, we've achieved a 90% compression ratio!
	//
	// As an aside, you might be thinking that typical JPEG compression is 90%, so why not JPEG encode
	// every frame? While true, the algorithm we have supplied above is quite a bit simpler than JPEG.
	// We demonstrate that taking advantage of temporal locality can yield compression ratios just as
	// high as JPEG, but with a much simpler algorithm.
	//
	// Additionally, the DEFLATE algorithm does not take advantage of the two dimensionality of the data
	// and is therefore not as efficient as it could be. In the real world, video codecs are much more
	// complex than the one we have implemented here. They take advantage of the two dimensionality of
	// the data, they use more sophisticated algorithms, and they are optimized for the hardware they
	// run on. For example, the H.264 codec is implemented in hardware on many modern GPUs.
	//
	// Now we have our encoded video. Let's decode it and see what we get.

	// First, we will decode the DEFLATE stream.
	var inflated bytes.Buffer
	r := flate.NewReader(&deflated)
	if _, err := io.Copy(&inflated, r); err != nil {
		log.Fatal(err)
	}
	if err := r.Close(); err != nil {
		log.Fatal(err)
	}

	// Split the inflated stream into frames.
	decodedFrames := make([][]byte, 0)
	for {
		frame := make([]byte, width*height*3/2)
		if _, err := io.ReadFull(&inflated, frame); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		decodedFrames = append(decodedFrames, frame)
	}

	// For every frame except the first one, we need to add the previous frame to the delta frame.
	// This is the opposite of what we did in the encoder.
	for i := range decodedFrames {
		if i == 0 {
			continue
		}

		for j := 0; j < len(decodedFrames[i]); j++ {
			decodedFrames[i][j] += decodedFrames[i-1][j]
		}
	}

	if err := os.WriteFile("decoded.yuv", bytes.Join(decodedFrames, nil), 0644); err != nil {
		log.Fatal(err)
	}

	// Then convert each YUV frame into RGB.
	for i, frame := range decodedFrames {
		Y := frame[:width*height]
		U := frame[width*height : width*height+width*height/4]
		V := frame[width*height+width*height/4:]

		rgb := make([]byte, 0, width*height*3)
		for j := 0; j < height; j++ {
			for k := 0; k < width; k++ {
				y := float64(Y[j*width+k])
				u := float64(U[(j/2)*(width/2)+(k/2)]) - 128
				v := float64(V[(j/2)*(width/2)+(k/2)]) - 128

				r := clamp(y+1.402*v, 0, 255)
				g := clamp(y-0.344*u-0.714*v, 0, 255)
				b := clamp(y+1.772*u, 0, 255)

				rgb = append(rgb, uint8(r), uint8(g), uint8(b))
			}
		}
		decodedFrames[i] = rgb
	}

	// Finally, write the decoded video to a file.
	//
	// This video can be played with ffplay:
	//
	//   ffplay -f rawvideo -pixel_format rgb24 -video_size 384x216 -framerate 25 decoded.rgb24
	//
	out, err := os.Create("decoded.rgb24")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	for i := range decodedFrames {
		if _, err := out.Write(decodedFrames[i]); err != nil {
			log.Fatal(err)
		}
	}
}

func size(frames [][]byte) int {
	var size int
	for _, frame := range frames {
		size += len(frame)
	}
	return size
}

func clamp(x, min, max float64) float64 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
