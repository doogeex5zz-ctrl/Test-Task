package main

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	filename := "C9200_9300_9400_9500_9600_cat9k_iosxe.16.12.07.SPA.bin"
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Не вдалося відкрити файл прошивки: ", err)
	}
	defer file.Close()

	// --- 1. ПАРСИНГ ЯДРА ---
	kernelStart := int64(0x0810)

	file.Seek(kernelStart+0x01F1, io.SeekStart)
	var setupSects uint8
	binary.Read(file, binary.LittleEndian, &setupSects)
	if setupSects == 0 {
		setupSects = 4
	}
	setupSize := int64(setupSects+1) * 512

	file.Seek(kernelStart+0x01F4, io.SeekStart)
	var sysSize uint32
	binary.Read(file, binary.LittleEndian, &sysSize)
	payloadSize := int64(sysSize) * 16

	totalKernelSize := setupSize + payloadSize
	initramfsStart := kernelStart + totalKernelSize

	fmt.Printf("[ SECTION 1 ] Cisco Boot Header\n")
	fmt.Printf("   - Offset:  0x00000000 -> 0x%08X\n", kernelStart-1)
	fmt.Printf("   - Contains certificates and metadata\n\n")

	fmt.Printf("[ SECTION 2 ] Linux kernel (bzImage)\n")
	fmt.Printf("   - Offset: 0x%08X -> 0x%08X\n", kernelStart, initramfsStart-1)
	fmt.Printf("   - Size:   %d bytes\n", totalKernelSize)

	// --- 2. ЕКСТРАКЦІЯ INITRAMFS ---
	fmt.Printf("[ SECTION 3 ] Initial filesystem (GZIP initramfs)\n")
	fmt.Printf("   - Offset: 0x%08X\n", initramfsStart)
	fmt.Printf("   - Action: Unpacking GZIP\n")

	file.Seek(initramfsStart, io.SeekStart)
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		log.Fatal("Error GZIP: ", err)
	}

	cpioFile, err := os.Create("1_extracted_initramfs.cpio")
	if err != nil {
		log.Fatal(err)
	}

	// Розпаковуємо initramfs
	cpioBytes, _ := io.Copy(cpioFile, gzipReader)
	cpioFile.Close()
	gzipReader.Close()

	fmt.Printf("   - Status: Save %d bytes in '1_extracted_initramfs.cpio'\n\n", cpioBytes)

	// --- 3. ЕКСТРАКЦІЯ SQUASHFS ---
	// Вирахував цю адресу в попередньому кроці
	squashfsStart := int64(41781497)

	fmt.Printf("[ SECTION 4 ] Primary file system (SquashFS / Super Package)\n")
	fmt.Printf("   - Offset: 0x%08X (Mathematically calculated)\n", squashfsStart)
	fmt.Printf("   - Action: Extracting the file system\n")

	// Перевіряю магічні байти для підтвердження
	file.Seek(squashfsStart, io.SeekStart)
	magic := make([]byte, 4)
	file.Read(magic)

	if string(magic) == "hsqs" {
		fmt.Printf("   - Check:  Magic bytes hsqs found\n")
	}

	squashFile, err := os.Create("2_filesystem.squashfs")
	if err != nil {
		log.Fatal(err)
	}

	// Копіюю всі дані від SquashFS до самого кінця файлу
	file.Seek(squashfsStart, io.SeekStart)
	squashBytes, _ := io.Copy(squashFile, file)
	squashFile.Close()

	fmt.Printf("   - Status: Save %d bytes in '2_filesystem.squashfs'\n", squashBytes)
	fmt.Println("======================================================")
	fmt.Println("Extraction successfully completed")
}
