# Domain Architecture: cmd/asm

## Layout Topology
```text
cmd/asm/
├── internal
│   ├── arch
│   │   ├── arch.go
│   │   ├── arm.go
│   │   ├── arm64.go
│   │   ├── loong64.go
│   │   ├── mips.go
│   │   ├── ppc64.go
│   │   ├── riscv64.go
│   │   └── s390x.go
│   ├── asm
│   │   ├── asm.go
│   │   └── parse.go
│   ├── flags
│   │   └── flags.go
│   └── lex
│       ├── input.go
│       ├── lex.go
│       ├── slice.go
│       ├── stack.go
│       └── tokenizer.go
├── doc.go
└── main.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/asm/doc.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Asm, typically invoked as “go tool asm”, assembles the source file into an object
file named for the basename of the argument source file with a .o suffix. The
object file can then be combined with other objects into a package archive.

# Command Line

Usage:

	go tool asm [flags] file

The specified file must be a Go assembly file.
The same assembler is used for all target operating systems and architectures.
The GOOS and GOARCH environment variables set the desired target.

Flags:

	-D name[=value]
		Predefine symbol name with an optional simple value.
		Can be repeated to define multiple symbols.
	-I dir1 -I dir2
		Search for #include files in dir1, dir2, etc,
		after consulting $GOROOT/pkg/$GOOS_$GOARCH.
	-S
		Print assembly and machine code.
	-V
		Print assembler version and exit.
	-debug
		Dump instructions as they are parsed.
	-dynlink
		Support references to Go symbols defined in other shared libraries.
	-e
		No limit on number of errors reported.
	-gensymabis
		Write symbol ABI information to output file. Don't assemble.
	-o file
		Write output to file. The default is foo.o for /a/b/c/foo.s.
	-p pkgpath
		Set expected package import to pkgpath.
	-shared
		Generate code that can be linked into a shared library.
	-spectre list
		Enable spectre mitigations in list (all, ret).
	-trimpath prefix
		Remove prefix from recorded source file paths.
	-v
		Print debug output.

Input language:

The assembler uses mostly the same syntax for all architectures,
the main variation having to do with addressing modes. Input is
run through a simplified C preprocessor that implements #include,
#define, #ifdef/endif, but not #if or ##.

For more information, see https://golang.org/doc/asm.
*/
package main

```

// === FILE: references!/go/src/cmd/asm/internal/arch/arch.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package arch defines architecture-specific information and support functions.
package arch

import (
	"cmd/internal/obj"
	"cmd/internal/obj/arm"
	"cmd/internal/obj/arm64"
	"cmd/internal/obj/loong64"
	"cmd/internal/obj/mips"
	"cmd/internal/obj/ppc64"
	"cmd/internal/obj/riscv"
	"cmd/internal/obj/s390x"
	"cmd/internal/obj/wasm"
	"cmd/internal/obj/x86"
	"fmt"
	"strings"
)

// Pseudo-registers whose names are the constant name without the leading R.
const (
	RFP = -(iota + 1)
	RSB
	RSP
	RPC
)

// Arch wraps the link architecture object with more architecture-specific information.
type Arch struct {
	*obj.LinkArch
	// Map of instruction names to enumeration.
	Instructions map[string]obj.As
	// Map of register names to enumeration.
	Register map[string]int16
	// Table of register prefix names. These are things like R for R(0) and SPR for SPR(268).
	RegisterPrefix map[string]bool
	// RegisterNumber converts R(10) into arm.REG_R10.
	RegisterNumber func(string, int16) (int16, bool)
	// Instruction is a jump.
	IsJump func(word string) bool
}

// nilRegisterNumber is the register number function for architectures
// that do not accept the R(N) notation. It always returns failure.
func nilRegisterNumber(name string, n int16) (int16, bool) {
	return 0, false
}

// Set configures the architecture specified by GOARCH and returns its representation.
// It returns nil if GOARCH is not recognized.
func Set(GOARCH string, shared bool) *Arch {
	switch GOARCH {
	case "386":
		return archX86(&x86.Link386)
	case "amd64":
		return archX86(&x86.Linkamd64)
	case "arm":
		return archArm()
	case "arm64":
		return archArm64()
	case "loong64":
		return archLoong64(&loong64.Linkloong64)
	case "mips":
		return archMips(&mips.Linkmips)
	case "mipsle":
		return archMips(&mips.Linkmipsle)
	case "mips64":
		return archMips64(&mips.Linkmips64)
	case "mips64le":
		return archMips64(&mips.Linkmips64le)
	case "ppc64":
		return archPPC64(&ppc64.Linkppc64)
	case "ppc64le":
		return archPPC64(&ppc64.Linkppc64le)
	case "riscv64":
		return archRISCV64(shared)
	case "s390x":
		return archS390x()
	case "wasm":
		return archWasm()
	}
	return nil
}

func jumpX86(word string) bool {
	return word[0] == 'J' || word == "CALL" || strings.HasPrefix(word, "LOOP") || word == "XBEGIN"
}

func jumpRISCV(word string) bool {
	switch word {
	case "BEQ", "BEQZ", "BGE", "BGEU", "BGEZ", "BGT", "BGTU", "BGTZ", "BLE", "BLEU", "BLEZ",
		"BLT", "BLTU", "BLTZ", "BNE", "BNEZ", "CALL", "CBEQZ", "CBNEZ", "CJ", "CJALR", "CJR",
		"JAL", "JALR", "JMP":
		return true
	}
	return false
}

func jumpWasm(word string) bool {
	return word == "JMP" || word == "CALL" || word == "Call" || word == "Br" || word == "BrIf"
}

func archX86(linkArch *obj.LinkArch) *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	for i, s := range x86.Register {
		register[s] = int16(i + x86.REG_AL)
	}
	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	if linkArch == &x86.Linkamd64 {
		// Alias g to R14
		register["g"] = x86.REGG
	}
	// Register prefix not used on this architecture.

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range x86.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseAMD64
		}
	}
	// Annoying aliases.
	instructions["JA"] = x86.AJHI   /* alternate */
	instructions["JAE"] = x86.AJCC  /* alternate */
	instructions["JB"] = x86.AJCS   /* alternate */
	instructions["JBE"] = x86.AJLS  /* alternate */
	instructions["JC"] = x86.AJCS   /* alternate */
	instructions["JCC"] = x86.AJCC  /* carry clear (CF = 0) */
	instructions["JCS"] = x86.AJCS  /* carry set (CF = 1) */
	instructions["JE"] = x86.AJEQ   /* alternate */
	instructions["JEQ"] = x86.AJEQ  /* equal (ZF = 1) */
	instructions["JG"] = x86.AJGT   /* alternate */
	instructions["JGE"] = x86.AJGE  /* greater than or equal (signed) (SF = OF) */
	instructions["JGT"] = x86.AJGT  /* greater than (signed) (ZF = 0 && SF = OF) */
	instructions["JHI"] = x86.AJHI  /* higher (unsigned) (CF = 0 && ZF = 0) */
	instructions["JHS"] = x86.AJCC  /* alternate */
	instructions["JL"] = x86.AJLT   /* alternate */
	instructions["JLE"] = x86.AJLE  /* less than or equal (signed) (ZF = 1 || SF != OF) */
	instructions["JLO"] = x86.AJCS  /* alternate */
	instructions["JLS"] = x86.AJLS  /* lower or same (unsigned) (CF = 1 || ZF = 1) */
	instructions["JLT"] = x86.AJLT  /* less than (signed) (SF != OF) */
	instructions["JMI"] = x86.AJMI  /* negative (minus) (SF = 1) */
	instructions["JNA"] = x86.AJLS  /* alternate */
	instructions["JNAE"] = x86.AJCS /* alternate */
	instructions["JNB"] = x86.AJCC  /* alternate */
	instructions["JNBE"] = x86.AJHI /* alternate */
	instructions["JNC"] = x86.AJCC  /* alternate */
	instructions["JNE"] = x86.AJNE  /* not equal (ZF = 0) */
	instructions["JNG"] = x86.AJLE  /* alternate */
	instructions["JNGE"] = x86.AJLT /* alternate */
	instructions["JNL"] = x86.AJGE  /* alternate */
	instructions["JNLE"] = x86.AJGT /* alternate */
	instructions["JNO"] = x86.AJOC  /* alternate */
	instructions["JNP"] = x86.AJPC  /* alternate */
	instructions["JNS"] = x86.AJPL  /* alternate */
	instructions["JNZ"] = x86.AJNE  /* alternate */
	instructions["JO"] = x86.AJOS   /* alternate */
	instructions["JOC"] = x86.AJOC  /* overflow clear (OF = 0) */
	instructions["JOS"] = x86.AJOS  /* overflow set (OF = 1) */
	instructions["JP"] = x86.AJPS   /* alternate */
	instructions["JPC"] = x86.AJPC  /* parity clear (PF = 0) */
	instructions["JPE"] = x86.AJPS  /* alternate */
	instructions["JPL"] = x86.AJPL  /* non-negative (plus) (SF = 0) */
	instructions["JPO"] = x86.AJPC  /* alternate */
	instructions["JPS"] = x86.AJPS  /* parity set (PF = 1) */
	instructions["JS"] = x86.AJMI   /* alternate */
	instructions["JZ"] = x86.AJEQ   /* alternate */
	instructions["MASKMOVDQU"] = x86.AMASKMOVOU
	instructions["MOVD"] = x86.AMOVQ
	instructions["MOVDQ2Q"] = x86.AMOVQ
	instructions["MOVNTDQ"] = x86.AMOVNTO
	instructions["MOVOA"] = x86.AMOVO
	instructions["PSLLDQ"] = x86.APSLLO
	instructions["PSRLDQ"] = x86.APSRLO
	instructions["PADDD"] = x86.APADDL
	// Spellings originally used in CL 97235.
	instructions["MOVBELL"] = x86.AMOVBEL
	instructions["MOVBEQQ"] = x86.AMOVBEQ
	instructions["MOVBEWW"] = x86.AMOVBEW

	return &Arch{
		LinkArch:       linkArch,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: nil,
		RegisterNumber: nilRegisterNumber,
		IsJump:         jumpX86,
	}
}

func archArm() *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	// Note that there is no list of names as there is for x86.
	for i := arm.REG_R0; i < arm.REG_SPSR; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	// Avoid unintentionally clobbering g using R10.
	delete(register, "R10")
	register["g"] = arm.REG_R10
	for i := 0; i < 16; i++ {
		register[fmt.Sprintf("C%d", i)] = int16(i)
	}

	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	register["SP"] = RSP
	registerPrefix := map[string]bool{
		"F": true,
		"R": true,
	}

	// special operands for DMB/DSB instructions
	register["MB_SY"] = arm.REG_MB_SY
	register["MB_ST"] = arm.REG_MB_ST
	register["MB_ISH"] = arm.REG_MB_ISH
	register["MB_ISHST"] = arm.REG_MB_ISHST
	register["MB_NSH"] = arm.REG_MB_NSH
	register["MB_NSHST"] = arm.REG_MB_NSHST
	register["MB_OSH"] = arm.REG_MB_OSH
	register["MB_OSHST"] = arm.REG_MB_OSHST

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range arm.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseARM
		}
	}
	// Annoying aliases.
	instructions["B"] = obj.AJMP
	instructions["BL"] = obj.ACALL
	// MCR differs from MRC by the way fields of the word are encoded.
	// (Details in arm.go). Here we add the instruction so parse will find
	// it, but give it an opcode number known only to us.
	instructions["MCR"] = aMCR

	return &Arch{
		LinkArch:       &arm.Linkarm,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: registerPrefix,
		RegisterNumber: armRegisterNumber,
		IsJump:         jumpArm,
	}
}

func archArm64() *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	// Note that there is no list of names as there is for 386 and amd64.
	register[obj.Rconv(arm64.REGSP)] = int16(arm64.REGSP)
	for i := arm64.REG_R0; i <= arm64.REG_R31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	// Rename R18 to R18_PLATFORM to avoid accidental use.
	register["R18_PLATFORM"] = register["R18"]
	delete(register, "R18")
	for i := arm64.REG_F0; i <= arm64.REG_F31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := arm64.REG_V0; i <= arm64.REG_V31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := arm64.REG_Z0; i <= arm64.REG_Z31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := arm64.REG_P0; i <= arm64.REG_P15; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := arm64.REG_PN0; i <= arm64.REG_PN15; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	// System registers.
	for i := 0; i < len(arm64.SystemReg); i++ {
		register[arm64.SystemReg[i].Name] = arm64.SystemReg[i].Reg
	}

	register["LR"] = arm64.REGLINK

	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	register["SP"] = RSP
	// Avoid unintentionally clobbering g using R28.
	delete(register, "R28")
	register["g"] = arm64.REG_R28
	registerPrefix := map[string]bool{
		"F":  true,
		"R":  true,
		"V":  true,
		"Z":  true,
		"P":  true,
		"PN": true,
	}

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range arm64.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseARM64
		}
	}
	// Annoying aliases.
	instructions["B"] = arm64.AB
	instructions["BL"] = arm64.ABL

	return &Arch{
		LinkArch:       &arm64.Linkarm64,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: registerPrefix,
		RegisterNumber: arm64RegisterNumber,
		IsJump:         jumpArm64,
	}

}

func archPPC64(linkArch *obj.LinkArch) *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	// Note that there is no list of names as there is for x86.
	for i := ppc64.REG_R0; i <= ppc64.REG_R31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := ppc64.REG_F0; i <= ppc64.REG_F31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := ppc64.REG_V0; i <= ppc64.REG_V31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := ppc64.REG_VS0; i <= ppc64.REG_VS63; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := ppc64.REG_A0; i <= ppc64.REG_A7; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := ppc64.REG_CR0; i <= ppc64.REG_CR7; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := ppc64.REG_MSR; i <= ppc64.REG_CR; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := ppc64.REG_CR0LT; i <= ppc64.REG_CR7SO; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	register["CR"] = ppc64.REG_CR
	register["XER"] = ppc64.REG_XER
	register["LR"] = ppc64.REG_LR
	register["CTR"] = ppc64.REG_CTR
	register["FPSCR"] = ppc64.REG_FPSCR
	register["MSR"] = ppc64.REG_MSR
	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	// Avoid unintentionally clobbering g using R30.
	delete(register, "R30")
	register["g"] = ppc64.REG_R30
	registerPrefix := map[string]bool{
		"CR":  true,
		"F":   true,
		"R":   true,
		"SPR": true,
	}

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range ppc64.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABasePPC64
		}
	}
	// The opcodes generated by x/arch's ppc64map are listed in
	// a separate slice, add them too.
	for i, s := range ppc64.GenAnames {
		instructions[s] = obj.As(i) + ppc64.AFIRSTGEN
	}
	// Annoying aliases.
	instructions["BR"] = ppc64.ABR
	instructions["BL"] = ppc64.ABL

	return &Arch{
		LinkArch:       linkArch,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: registerPrefix,
		RegisterNumber: ppc64RegisterNumber,
		IsJump:         jumpPPC64,
	}
}

func archMips(linkArch *obj.LinkArch) *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	// Note that there is no list of names as there is for x86.
	for i := mips.REG_R0; i <= mips.REG_R31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	for i := mips.REG_F0; i <= mips.REG_F31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := mips.REG_M0; i <= mips.REG_M31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := mips.REG_FCR0; i <= mips.REG_FCR31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	register["HI"] = mips.REG_HI
	register["LO"] = mips.REG_LO
	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	// Avoid unintentionally clobbering g using R30.
	delete(register, "R30")
	register["g"] = mips.REG_R30

	registerPrefix := map[string]bool{
		"F":   true,
		"FCR": true,
		"M":   true,
		"R":   true,
	}

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range mips.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseMIPS
		}
	}
	// Annoying alias.
	instructions["JAL"] = mips.AJAL

	return &Arch{
		LinkArch:       linkArch,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: registerPrefix,
		RegisterNumber: mipsRegisterNumber,
		IsJump:         jumpMIPS,
	}
}

func archMips64(linkArch *obj.LinkArch) *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	// Note that there is no list of names as there is for x86.
	for i := mips.REG_R0; i <= mips.REG_R31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := mips.REG_F0; i <= mips.REG_F31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := mips.REG_M0; i <= mips.REG_M31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := mips.REG_FCR0; i <= mips.REG_FCR31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := mips.REG_W0; i <= mips.REG_W31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	register["HI"] = mips.REG_HI
	register["LO"] = mips.REG_LO
	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	// Avoid unintentionally clobbering g using R30.
	delete(register, "R30")
	register["g"] = mips.REG_R30
	// Avoid unintentionally clobbering RSB using R28.
	delete(register, "R28")
	register["RSB"] = mips.REG_R28
	registerPrefix := map[string]bool{
		"F":   true,
		"FCR": true,
		"M":   true,
		"R":   true,
		"W":   true,
	}

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range mips.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseMIPS
		}
	}
	// Annoying alias.
	instructions["JAL"] = mips.AJAL

	return &Arch{
		LinkArch:       linkArch,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: registerPrefix,
		RegisterNumber: mipsRegisterNumber,
		IsJump:         jumpMIPS,
	}
}

func archLoong64(linkArch *obj.LinkArch) *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	// Note that there is no list of names as there is for x86.
	for i := loong64.REG_R0; i <= loong64.REG_R31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	for i := loong64.REG_F0; i <= loong64.REG_F31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	for i := loong64.REG_FCSR0; i <= loong64.REG_FCSR31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	for i := loong64.REG_FCC0; i <= loong64.REG_FCC31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	for i := loong64.REG_V0; i <= loong64.REG_V31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	for i := loong64.REG_X0; i <= loong64.REG_X31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}

	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	// Avoid unintentionally clobbering g using R22.
	delete(register, "R22")
	register["g"] = loong64.REG_R22
	registerPrefix := map[string]bool{
		"F":    true,
		"FCSR": true,
		"FCC":  true,
		"R":    true,
		"V":    true,
		"X":    true,
	}

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range loong64.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseLoong64
		}
	}
	// Annoying alias.
	instructions["JAL"] = loong64.AJAL

	return &Arch{
		LinkArch:       linkArch,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: registerPrefix,
		RegisterNumber: loong64RegisterNumber,
		IsJump:         jumpLoong64,
	}
}

func archRISCV64(shared bool) *Arch {
	register := make(map[string]int16)

	// Standard register names.
	for i := riscv.REG_X0; i <= riscv.REG_X31; i++ {
		// Disallow X3 in shared mode, as this will likely be used as the
		// GP register, which could result in problems in non-Go code,
		// including signal handlers.
		if shared && i == riscv.REG_GP {
			continue
		}
		if i == riscv.REG_TP || i == riscv.REG_G {
			continue
		}
		name := fmt.Sprintf("X%d", i-riscv.REG_X0)
		register[name] = int16(i)
	}
	for i := riscv.REG_F0; i <= riscv.REG_F31; i++ {
		name := fmt.Sprintf("F%d", i-riscv.REG_F0)
		register[name] = int16(i)
	}
	for i := riscv.REG_V0; i <= riscv.REG_V31; i++ {
		name := fmt.Sprintf("V%d", i-riscv.REG_V0)
		register[name] = int16(i)
	}

	// General registers with ABI names.
	register["ZERO"] = riscv.REG_ZERO
	register["RA"] = riscv.REG_RA
	register["SP"] = riscv.REG_SP
	register["GP"] = riscv.REG_GP
	register["TP"] = riscv.REG_TP
	register["T0"] = riscv.REG_T0
	register["T1"] = riscv.REG_T1
	register["T2"] = riscv.REG_T2
	register["S0"] = riscv.REG_S0
	register["S1"] = riscv.REG_S1
	register["A0"] = riscv.REG_A0
	register["A1"] = riscv.REG_A1
	register["A2"] = riscv.REG_A2
	register["A3"] = riscv.REG_A3
	register["A4"] = riscv.REG_A4
	register["A5"] = riscv.REG_A5
	register["A6"] = riscv.REG_A6
	register["A7"] = riscv.REG_A7
	register["S2"] = riscv.REG_S2
	register["S3"] = riscv.REG_S3
	register["S4"] = riscv.REG_S4
	register["S5"] = riscv.REG_S5
	register["S6"] = riscv.REG_S6
	register["S7"] = riscv.REG_S7
	register["S8"] = riscv.REG_S8
	register["S9"] = riscv.REG_S9
	register["S10"] = riscv.REG_S10
	// Skip S11 as it is the g register.
	register["T3"] = riscv.REG_T3
	register["T4"] = riscv.REG_T4
	register["T5"] = riscv.REG_T5
	register["T6"] = riscv.REG_T6

	// Go runtime register names.
	register["g"] = riscv.REG_G
	register["CTXT"] = riscv.REG_CTXT
	register["TMP"] = riscv.REG_TMP

	// ABI names for floating point register.
	register["FT0"] = riscv.REG_FT0
	register["FT1"] = riscv.REG_FT1
	register["FT2"] = riscv.REG_FT2
	register["FT3"] = riscv.REG_FT3
	register["FT4"] = riscv.REG_FT4
	register["FT5"] = riscv.REG_FT5
	register["FT6"] = riscv.REG_FT6
	register["FT7"] = riscv.REG_FT7
	register["FS0"] = riscv.REG_FS0
	register["FS1"] = riscv.REG_FS1
	register["FA0"] = riscv.REG_FA0
	register["FA1"] = riscv.REG_FA1
	register["FA2"] = riscv.REG_FA2
	register["FA3"] = riscv.REG_FA3
	register["FA4"] = riscv.REG_FA4
	register["FA5"] = riscv.REG_FA5
	register["FA6"] = riscv.REG_FA6
	register["FA7"] = riscv.REG_FA7
	register["FS2"] = riscv.REG_FS2
	register["FS3"] = riscv.REG_FS3
	register["FS4"] = riscv.REG_FS4
	register["FS5"] = riscv.REG_FS5
	register["FS6"] = riscv.REG_FS6
	register["FS7"] = riscv.REG_FS7
	register["FS8"] = riscv.REG_FS8
	register["FS9"] = riscv.REG_FS9
	register["FS10"] = riscv.REG_FS10
	register["FS11"] = riscv.REG_FS11
	register["FT8"] = riscv.REG_FT8
	register["FT9"] = riscv.REG_FT9
	register["FT10"] = riscv.REG_FT10
	register["FT11"] = riscv.REG_FT11

	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range riscv.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseRISCV
		}
	}

	return &Arch{
		LinkArch:       &riscv.LinkRISCV64,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: nil,
		RegisterNumber: nilRegisterNumber,
		IsJump:         jumpRISCV,
	}
}

func archS390x() *Arch {
	register := make(map[string]int16)
	// Create maps for easy lookup of instruction names etc.
	// Note that there is no list of names as there is for x86.
	for i := s390x.REG_R0; i <= s390x.REG_R15; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := s390x.REG_F0; i <= s390x.REG_F15; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := s390x.REG_V0; i <= s390x.REG_V31; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	for i := s390x.REG_AR0; i <= s390x.REG_AR15; i++ {
		register[obj.Rconv(i)] = int16(i)
	}
	register["LR"] = s390x.REG_LR
	// Pseudo-registers.
	register["SB"] = RSB
	register["FP"] = RFP
	register["PC"] = RPC
	// Avoid unintentionally clobbering g using R13.
	delete(register, "R13")
	register["g"] = s390x.REG_R13
	registerPrefix := map[string]bool{
		"AR": true,
		"F":  true,
		"R":  true,
	}

	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range s390x.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseS390X
		}
	}
	// Annoying aliases.
	instructions["BR"] = s390x.ABR
	instructions["BL"] = s390x.ABL

	return &Arch{
		LinkArch:       &s390x.Links390x,
		Instructions:   instructions,
		Register:       register,
		RegisterPrefix: registerPrefix,
		RegisterNumber: s390xRegisterNumber,
		IsJump:         jumpS390x,
	}
}

func archWasm() *Arch {
	instructions := make(map[string]obj.As)
	for i, s := range obj.Anames {
		instructions[s] = obj.As(i)
	}
	for i, s := range wasm.Anames {
		if obj.As(i) >= obj.A_ARCHSPECIFIC {
			instructions[s] = obj.As(i) + obj.ABaseWasm
		}
	}

	return &Arch{
		LinkArch:       &wasm.Linkwasm,
		Instructions:   instructions,
		Register:       wasm.Register,
		RegisterPrefix: nil,
		RegisterNumber: nilRegisterNumber,
		IsJump:         jumpWasm,
	}
}

```

// === FILE: references!/go/src/cmd/asm/internal/arch/arm.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file encapsulates some of the odd characteristics of the ARM
// instruction set, to minimize its interaction with the core of the
// assembler.

package arch

import (
	"strings"

	"cmd/internal/obj"
	"cmd/internal/obj/arm"
)

var armLS = map[string]uint8{
	"U":  arm.C_UBIT,
	"S":  arm.C_SBIT,
	"W":  arm.C_WBIT,
	"P":  arm.C_PBIT,
	"PW": arm.C_WBIT | arm.C_PBIT,
	"WP": arm.C_WBIT | arm.C_PBIT,
}

var armSCOND = map[string]uint8{
	"EQ":  arm.C_SCOND_EQ,
	"NE":  arm.C_SCOND_NE,
	"CS":  arm.C_SCOND_HS,
	"HS":  arm.C_SCOND_HS,
	"CC":  arm.C_SCOND_LO,
	"LO":  arm.C_SCOND_LO,
	"MI":  arm.C_SCOND_MI,
	"PL":  arm.C_SCOND_PL,
	"VS":  arm.C_SCOND_VS,
	"VC":  arm.C_SCOND_VC,
	"HI":  arm.C_SCOND_HI,
	"LS":  arm.C_SCOND_LS,
	"GE":  arm.C_SCOND_GE,
	"LT":  arm.C_SCOND_LT,
	"GT":  arm.C_SCOND_GT,
	"LE":  arm.C_SCOND_LE,
	"AL":  arm.C_SCOND_NONE,
	"U":   arm.C_UBIT,
	"S":   arm.C_SBIT,
	"W":   arm.C_WBIT,
	"P":   arm.C_PBIT,
	"PW":  arm.C_WBIT | arm.C_PBIT,
	"WP":  arm.C_WBIT | arm.C_PBIT,
	"F":   arm.C_FBIT,
	"IBW": arm.C_WBIT | arm.C_PBIT | arm.C_UBIT,
	"IAW": arm.C_WBIT | arm.C_UBIT,
	"DBW": arm.C_WBIT | arm.C_PBIT,
	"DAW": arm.C_WBIT,
	"IB":  arm.C_PBIT | arm.C_UBIT,
	"IA":  arm.C_UBIT,
	"DB":  arm.C_PBIT,
	"DA":  0,
}

var armJump = map[string]bool{
	"B":    true,
	"BL":   true,
	"BX":   true,
	"BEQ":  true,
	"BNE":  true,
	"BCS":  true,
	"BHS":  true,
	"BCC":  true,
	"BLO":  true,
	"BMI":  true,
	"BPL":  true,
	"BVS":  true,
	"BVC":  true,
	"BHI":  true,
	"BLS":  true,
	"BGE":  true,
	"BLT":  true,
	"BGT":  true,
	"BLE":  true,
	"CALL": true,
	"JMP":  true,
}

func jumpArm(word string) bool {
	return armJump[word]
}

// IsARMCMP reports whether the op (as defined by an arm.A* constant) is
// one of the comparison instructions that require special handling.
func IsARMCMP(op obj.As) bool {
	switch op {
	case arm.ACMN, arm.ACMP, arm.ATEQ, arm.ATST:
		return true
	}
	return false
}

// IsARMSTREX reports whether the op (as defined by an arm.A* constant) is
// one of the STREX-like instructions that require special handling.
func IsARMSTREX(op obj.As) bool {
	switch op {
	case arm.ASTREX, arm.ASTREXD, arm.ASTREXB, arm.ASWPW, arm.ASWPBU:
		return true
	}
	return false
}

// MCR is not defined by the obj/arm; instead we define it privately here.
// It is encoded as an MRC with a bit inside the instruction word,
// passed to arch.ARMMRCOffset.
const aMCR = arm.ALAST + 1

// IsARMMRC reports whether the op (as defined by an arm.A* constant) is
// MRC or MCR.
func IsARMMRC(op obj.As) bool {
	switch op {
	case arm.AMRC, aMCR: // Note: aMCR is defined in this package.
		return true
	}
	return false
}

// IsARMBFX reports whether the op (as defined by an arm.A* constant) is one the
// BFX-like instructions which are in the form of "op $width, $LSB, (Reg,) Reg".
func IsARMBFX(op obj.As) bool {
	switch op {
	case arm.ABFX, arm.ABFXU, arm.ABFC, arm.ABFI:
		return true
	}
	return false
}

// IsARMFloatCmp reports whether the op is a floating comparison instruction.
func IsARMFloatCmp(op obj.As) bool {
	switch op {
	case arm.ACMPF, arm.ACMPD:
		return true
	}
	return false
}

// ARMMRCOffset implements the peculiar encoding of the MRC and MCR instructions.
// The difference between MRC and MCR is represented by a bit high in the word, not
// in the usual way by the opcode itself. Asm must use AMRC for both instructions, so
// we return the opcode for MRC so that asm doesn't need to import obj/arm.
func ARMMRCOffset(op obj.As, cond string, x0, x1, x2, x3, x4, x5 int64) (offset int64, op0 obj.As, ok bool) {
	op1 := int64(0)
	if op == arm.AMRC {
		op1 = 1
	}
	bits, ok := ParseARMCondition(cond)
	if !ok {
		return
	}
	offset = (0xe << 24) | // opcode
		(op1 << 20) | // MCR/MRC
		((int64(bits) ^ arm.C_SCOND_XOR) << 28) | // scond
		((x0 & 15) << 8) | //coprocessor number
		((x1 & 7) << 21) | // coprocessor operation
		((x2 & 15) << 12) | // ARM register
		((x3 & 15) << 16) | // Crn
		((x4 & 15) << 0) | // Crm
		((x5 & 7) << 5) | // coprocessor information
		(1 << 4) /* must be set */
	return offset, arm.AMRC, true
}

// IsARMMULA reports whether the op (as defined by an arm.A* constant) is
// MULA, MULS, MMULA, MMULS, MULABB, MULAWB or MULAWT, the 4-operand instructions.
func IsARMMULA(op obj.As) bool {
	switch op {
	case arm.AMULA, arm.AMULS, arm.AMMULA, arm.AMMULS, arm.AMULABB, arm.AMULAWB, arm.AMULAWT:
		return true
	}
	return false
}

var bcode = []obj.As{
	arm.ABEQ,
	arm.ABNE,
	arm.ABCS,
	arm.ABCC,
	arm.ABMI,
	arm.ABPL,
	arm.ABVS,
	arm.ABVC,
	arm.ABHI,
	arm.ABLS,
	arm.ABGE,
	arm.ABLT,
	arm.ABGT,
	arm.ABLE,
	arm.AB,
	obj.ANOP,
}

// ARMConditionCodes handles the special condition code situation for the ARM.
// It returns a boolean to indicate success; failure means cond was unrecognized.
func ARMConditionCodes(prog *obj.Prog, cond string) bool {
	if cond == "" {
		return true
	}
	bits, ok := ParseARMCondition(cond)
	if !ok {
		return false
	}
	/* hack to make B.NE etc. work: turn it into the corresponding conditional */
	if prog.As == arm.AB {
		prog.As = bcode[(bits^arm.C_SCOND_XOR)&0xf]
		bits = (bits &^ 0xf) | arm.C_SCOND_NONE
	}
	prog.Scond = bits
	return true
}

// ParseARMCondition parses the conditions attached to an ARM instruction.
// The input is a single string consisting of period-separated condition
// codes, such as ".P.W". An initial period is ignored.
func ParseARMCondition(cond string) (uint8, bool) {
	return parseARMCondition(cond, armLS, armSCOND)
}

func parseARMCondition(cond string, ls, scond map[string]uint8) (uint8, bool) {
	cond = strings.TrimPrefix(cond, ".")
	if cond == "" {
		return arm.C_SCOND_NONE, true
	}
	names := strings.Split(cond, ".")
	bits := uint8(0)
	for _, name := range names {
		if b, present := ls[name]; present {
			bits |= b
			continue
		}
		if b, present := scond[name]; present {
			bits = (bits &^ arm.C_SCOND) | b
			continue
		}
		return 0, false
	}
	return bits, true
}

func armRegisterNumber(name string, n int16) (int16, bool) {
	if n < 0 || 15 < n {
		return 0, false
	}
	switch name {
	case "R":
		return arm.REG_R0 + n, true
	case "F":
		return arm.REG_F0 + n, true
	}
	return 0, false
}

```

// === FILE: references!/go/src/cmd/asm/internal/arch/arm64.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file encapsulates some of the odd characteristics of the ARM64
// instruction set, to minimize its interaction with the core of the
// assembler.

package arch

import (
	"cmd/internal/obj"
	"cmd/internal/obj/arm64"
	"errors"
)

var arm64LS = map[string]uint8{
	"P": arm64.C_XPOST,
	"W": arm64.C_XPRE,
}

var arm64Jump = map[string]bool{
	"B":     true,
	"BL":    true,
	"BEQ":   true,
	"BNE":   true,
	"BCS":   true,
	"BHS":   true,
	"BCC":   true,
	"BLO":   true,
	"BMI":   true,
	"BPL":   true,
	"BVS":   true,
	"BVC":   true,
	"BHI":   true,
	"BLS":   true,
	"BGE":   true,
	"BLT":   true,
	"BGT":   true,
	"BLE":   true,
	"CALL":  true,
	"CBZ":   true,
	"CBZW":  true,
	"CBNZ":  true,
	"CBNZW": true,
	"JMP":   true,
	"TBNZ":  true,
	"TBZ":   true,

	// ADR isn't really a jump, but it takes a PC or label reference,
	// which needs to patched like a jump.
	"ADR":  true,
	"ADRP": true,
}

func jumpArm64(word string) bool {
	return arm64Jump[word]
}

var arm64SpecialOperand map[string]arm64.SpecialOperand

// ARM64SpecialOperand returns the internal representation of a special operand.
func ARM64SpecialOperand(name string) arm64.SpecialOperand {
	if arm64SpecialOperand == nil {
		// Generate mapping when function is first called.
		arm64SpecialOperand = map[string]arm64.SpecialOperand{}
		for opd := arm64.SPOP_BEGIN; opd < arm64.SPOP_END; opd++ {
			arm64SpecialOperand[opd.String()] = opd
		}

		// Handle some special cases.
		specialMapping := map[string]arm64.SpecialOperand{
			// The internal representation of CS(CC) and HS(LO) are the same.
			"CS": arm64.SPOP_HS,
			"CC": arm64.SPOP_LO,
		}
		for s, opd := range specialMapping {
			arm64SpecialOperand[s] = opd
		}
	}
	if opd, ok := arm64SpecialOperand[name]; ok {
		return opd
	}
	return arm64.SPOP_END
}

// IsARM64ADR reports whether the op (as defined by an arm64.A* constant) is
// one of the comparison instructions that require special handling.
func IsARM64ADR(op obj.As) bool {
	switch op {
	case arm64.AADR, arm64.AADRP:
		return true
	}
	return false
}

// IsARM64CMP reports whether the op (as defined by an arm64.A* constant) is
// one of the comparison instructions that require special handling.
func IsARM64CMP(op obj.As) bool {
	switch op {
	case arm64.ACMN, arm64.ACMP, arm64.ATST,
		arm64.ACMNW, arm64.ACMPW, arm64.ATSTW,
		arm64.AFCMPS, arm64.AFCMPD,
		arm64.AFCMPES, arm64.AFCMPED:
		return true
	}
	return false
}

// IsARM64STLXR reports whether the op (as defined by an arm64.A*
// constant) is one of the STLXR-like instructions that require special
// handling.
func IsARM64STLXR(op obj.As) bool {
	switch op {
	case arm64.ASTLXRB, arm64.ASTLXRH, arm64.ASTLXRW, arm64.ASTLXR,
		arm64.ASTXRB, arm64.ASTXRH, arm64.ASTXRW, arm64.ASTXR,
		arm64.ASTXP, arm64.ASTXPW, arm64.ASTLXP, arm64.ASTLXPW:
		return true
	}
	// LDADDx/SWPx/CASx atomic instructions
	return arm64.IsAtomicInstruction(op)
}

// IsARM64TBL reports whether the op (as defined by an arm64.A*
// constant) is one of the TBL-like instructions and one of its
// inputs does not fit into prog.Reg, so require special handling.
func IsARM64TBL(op obj.As) bool {
	switch op {
	case arm64.AVTBL, arm64.AVTBX, arm64.AVMOVQ:
		return true
	}
	return false
}

// IsARM64CASP reports whether the op (as defined by an arm64.A*
// constant) is one of the CASP-like instructions, and its 2nd
// destination is a register pair that require special handling.
func IsARM64CASP(op obj.As) bool {
	switch op {
	case arm64.ACASPD, arm64.ACASPW:
		return true
	}
	return false
}

// ARM64Suffix handles the special suffix for the ARM64.
// It returns a boolean to indicate success; failure means
// cond was unrecognized.
func ARM64Suffix(prog *obj.Prog, cond string) bool {
	if cond == "" {
		return true
	}
	bits, ok := parseARM64Suffix(cond)
	if !ok {
		return false
	}
	prog.Scond = bits
	return true
}

// parseARM64Suffix parses the suffix attached to an ARM64 instruction.
// The input is a single string consisting of period-separated condition
// codes, such as ".P.W". An initial period is ignored.
func parseARM64Suffix(cond string) (uint8, bool) {
	if cond == "" {
		return 0, true
	}
	return parseARMCondition(cond, arm64LS, nil)
}

func arm64RegisterNumber(name string, n int16) (int16, bool) {
	switch name {
	case "F":
		if 0 <= n && n <= 31 {
			return arm64.REG_F0 + n, true
		}
	case "R":
		if 0 <= n && n <= 30 { // not 31
			return arm64.REG_R0 + n, true
		}
	case "V":
		if 0 <= n && n <= 31 {
			return arm64.REG_V0 + n, true
		}
	case "Z":
		if 0 <= n && n <= 31 {
			return arm64.REG_Z0 + n, true
		}
	case "P":
		if 0 <= n && n <= 15 {
			return arm64.REG_P0 + n, true
		}
	case "PN":
		if 0 <= n && n <= 15 {
			return arm64.REG_PN0 + n, true
		}
	}
	return 0, false
}

// ARM64RegisterShift constructs an ARM64 register with shift operation.
func ARM64RegisterShift(reg, op, count int16) (int64, error) {
	// the base register of shift operations must be general register.
	if reg > arm64.REG_R31 || reg < arm64.REG_R0 {
		return 0, errors.New("invalid register for shift operation")
	}
	return int64(reg&31)<<16 | int64(op)<<22 | int64(uint16(count)), nil
}

// ARM64RegisterArrangement constructs an ARM64 vector register arrangement.
func ARM64RegisterArrangement(reg int16, name, arng string) (int64, error) {
	var curQ, curSize, prefix uint16
	if name[0] != 'V' && name[0] != 'Z' && name[0] != 'P' {
		return 0, errors.New("expect V0-V31, Z0-Z31, or P0-P15; found: " + name)
	}
	switch name[0] {
	case 'V':
		prefix = 0
	case 'Z':
		prefix = 1
	case 'P':
		prefix = 2
	}
	if reg < 0 {
		return 0, errors.New("invalid register number: " + name)
	}
	switch arng {
	case "B8":
		curSize = 0
		curQ = 0
	case "B16":
		curSize = 0
		curQ = 1
	case "H4":
		curSize = 1
		curQ = 0
	case "H8":
		curSize = 1
		curQ = 1
	case "S2":
		curSize = 2
		curQ = 0
	case "S4":
		curSize = 2
		curQ = 1
	case "D1":
		curSize = 3
		curQ = 0
	case "D2":
		curSize = 3
		curQ = 1
	case "B":
		curSize = 1
		curQ = 2
	case "H":
		curSize = 2
		curQ = 2
	case "S":
		curSize = 3
		curQ = 2
	case "D":
		curSize = 1
		curQ = 3
	case "Q":
		curSize = 2
		curQ = 3
	default:
		return 0, errors.New("invalid arrangement in ARM64 register list")
	}
	return (int64(prefix) << 32) | (int64(curQ) & 3 << 30) | (int64(curSize&3) << 10), nil
}

```

// === FILE: references!/go/src/cmd/asm/internal/arch/loong64.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file encapsulates some of the odd characteristics of the
// Loong64 (LoongArch64) instruction set, to minimize its interaction
// with the core of the assembler.

package arch

import (
	"cmd/internal/obj"
	"cmd/internal/obj/loong64"
	"errors"
	"fmt"
)

func jumpLoong64(word string) bool {
	switch word {
	case "BEQ", "BFPF", "BFPT", "BLTZ", "BGEZ", "BLEZ", "BGTZ", "BLT", "BLTU", "JIRL", "BNE", "BGE", "BGEU", "JMP", "JAL", "CALL":
		return true
	}
	return false
}

// IsLoong64RDTIME reports whether the op (as defined by an loong64.A*
// constant) is one of the RDTIMELW/RDTIMEHW/RDTIMED instructions that
// require special handling.
func IsLoong64RDTIME(op obj.As) bool {
	switch op {
	case loong64.ARDTIMELW, loong64.ARDTIMEHW, loong64.ARDTIMED:
		return true
	}
	return false
}

func IsLoong64PRELD(op obj.As) bool {
	switch op {
	case loong64.APRELD, loong64.APRELDX:
		return true
	}
	return false
}

func IsLoong64AMO(op obj.As) bool {
	return loong64.IsAtomicInst(op)
}

var loong64ElemExtMap = map[string]int16{
	"B":  loong64.ARNG_B,
	"H":  loong64.ARNG_H,
	"W":  loong64.ARNG_W,
	"V":  loong64.ARNG_V,
	"BU": loong64.ARNG_BU,
	"HU": loong64.ARNG_HU,
	"WU": loong64.ARNG_WU,
	"VU": loong64.ARNG_VU,
}

var loong64LsxArngExtMap = map[string]int16{
	"B16": loong64.ARNG_16B,
	"H8":  loong64.ARNG_8H,
	"W4":  loong64.ARNG_4W,
	"V2":  loong64.ARNG_2V,
}

var loong64LasxArngExtMap = map[string]int16{
	"B32": loong64.ARNG_32B,
	"H16": loong64.ARNG_16H,
	"W8":  loong64.ARNG_8W,
	"V4":  loong64.ARNG_4V,
	"Q2":  loong64.ARNG_2Q,
}

// Loong64RegisterExtension constructs an Loong64 register with extension or arrangement.
func Loong64RegisterExtension(a *obj.Addr, ext string, reg, num int16, isAmount, isIndex bool) error {
	var ok bool
	var arngType int16
	var simdType int16
	var simdReg int16

	switch {
	case reg >= loong64.REG_V0 && reg <= loong64.REG_V31:
		simdType = loong64.LSX
		simdReg = reg - loong64.REG_V0
	case reg >= loong64.REG_X0 && reg <= loong64.REG_X31:
		simdType = loong64.LASX
		simdReg = reg - loong64.REG_X0
	default:
		return errors.New("Loong64 extension: invalid LSX/LASX register: " + fmt.Sprintf("%d", reg))
	}

	if isIndex {
		arngType, ok = loong64ElemExtMap[ext]
		if !ok {
			return errors.New("Loong64 extension: invalid LSX/LASX arrangement type: " + ext)
		}

		a.Reg = loong64.REG_ELEM
		a.Reg += ((simdReg & loong64.EXT_REG_MASK) << loong64.EXT_REG_SHIFT)
		a.Reg += ((arngType & loong64.EXT_TYPE_MASK) << loong64.EXT_TYPE_SHIFT)
		a.Reg += ((simdType & loong64.EXT_SIMDTYPE_MASK) << loong64.EXT_SIMDTYPE_SHIFT)
		a.Index = num
	} else {
		switch simdType {
		case loong64.LSX:
			arngType, ok = loong64LsxArngExtMap[ext]
			if !ok {
				return errors.New("Loong64 extension: invalid LSX arrangement type: " + ext)
			}

		case loong64.LASX:
			arngType, ok = loong64LasxArngExtMap[ext]
			if !ok {
				return errors.New("Loong64 extension: invalid LASX arrangement type: " + ext)
			}
		}

		a.Reg = loong64.REG_ARNG
		a.Reg += ((simdReg & loong64.EXT_REG_MASK) << loong64.EXT_REG_SHIFT)
		a.Reg += ((arngType & loong64.EXT_TYPE_MASK) << loong64.EXT_TYPE_SHIFT)
		a.Reg += ((simdType & loong64.EXT_SIMDTYPE_MASK) << loong64.EXT_SIMDTYPE_SHIFT)
	}

	return nil
}

func loong64RegisterNumber(name string, n int16) (int16, bool) {
	switch name {
	case "F":
		if 0 <= n && n <= 31 {
			return loong64.REG_F0 + n, true
		}
	case "FCSR":
		if 0 <= n && n <= 31 {
			return loong64.REG_FCSR0 + n, true
		}
	case "FCC":
		if 0 <= n && n <= 31 {
			return loong64.REG_FCC0 + n, true
		}
	case "R":
		if 0 <= n && n <= 31 {
			return loong64.REG_R0 + n, true
		}
	case "V":
		if 0 <= n && n <= 31 {
			return loong64.REG_V0 + n, true
		}
	case "X":
		if 0 <= n && n <= 31 {
			return loong64.REG_X0 + n, true
		}
	}
	return 0, false
}

```

// === FILE: references!/go/src/cmd/asm/internal/arch/mips.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file encapsulates some of the odd characteristics of the
// MIPS (MIPS64) instruction set, to minimize its interaction
// with the core of the assembler.

package arch

import (
	"cmd/internal/obj"
	"cmd/internal/obj/mips"
)

func jumpMIPS(word string) bool {
	switch word {
	case "BEQ", "BFPF", "BFPT", "BGEZ", "BGEZAL", "BGTZ", "BLEZ", "BLTZ", "BLTZAL", "BNE", "JMP", "JAL", "CALL":
		return true
	}
	return false
}

// IsMIPSCMP reports whether the op (as defined by an mips.A* constant) is
// one of the CMP instructions that require special handling.
func IsMIPSCMP(op obj.As) bool {
	switch op {
	case mips.ACMPEQF, mips.ACMPEQD, mips.ACMPGEF, mips.ACMPGED,
		mips.ACMPGTF, mips.ACMPGTD:
		return true
	}
	return false
}

// IsMIPSMUL reports whether the op (as defined by an mips.A* constant) is
// one of the MUL/DIV/REM/MADD/MSUB instructions that require special handling.
func IsMIPSMUL(op obj.As) bool {
	switch op {
	case mips.AMUL, mips.AMULU, mips.AMULV, mips.AMULVU,
		mips.ADIV, mips.ADIVU, mips.ADIVV, mips.ADIVVU,
		mips.AREM, mips.AREMU, mips.AREMV, mips.AREMVU,
		mips.AMADD, mips.AMSUB:
		return true
	}
	return false
}

func mipsRegisterNumber(name string, n int16) (int16, bool) {
	switch name {
	case "F":
		if 0 <= n && n <= 31 {
			return mips.REG_F0 + n, true
		}
	case "FCR":
		if 0 <= n && n <= 31 {
			return mips.REG_FCR0 + n, true
		}
	case "M":
		if 0 <= n && n <= 31 {
			return mips.REG_M0 + n, true
		}
	case "R":
		if 0 <= n && n <= 31 {
			return mips.REG_R0 + n, true
		}
	case "W":
		if 0 <= n && n <= 31 {
			return mips.REG_W0 + n, true
		}
	}
	return 0, false
}

```

// === FILE: references!/go/src/cmd/asm/internal/arch/ppc64.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file encapsulates some of the odd characteristics of the
// 64-bit PowerPC (PPC64) instruction set, to minimize its interaction
// with the core of the assembler.

package arch

import (
	"cmd/internal/obj"
	"cmd/internal/obj/ppc64"
)

func jumpPPC64(word string) bool {
	switch word {
	case "BC", "BCL", "BEQ", "BGE", "BGT", "BL", "BLE", "BLT", "BNE", "BR", "BVC", "BVS", "BDNZ", "BDZ", "CALL", "JMP":
		return true
	}
	return false
}

// IsPPC64CMP reports whether the op (as defined by an ppc64.A* constant) is
// one of the CMP instructions that require special handling.
func IsPPC64CMP(op obj.As) bool {
	switch op {
	case ppc64.ACMP, ppc64.ACMPU, ppc64.ACMPW, ppc64.ACMPWU, ppc64.AFCMPO, ppc64.AFCMPU, ppc64.ADCMPO, ppc64.ADCMPU, ppc64.ADCMPOQ, ppc64.ADCMPUQ:
		return true
	}
	return false
}

// IsPPC64NEG reports whether the op (as defined by an ppc64.A* constant) is
// one of the NEG-like instructions that require special handling.
func IsPPC64NEG(op obj.As) bool {
	switch op {
	case ppc64.AADDMECC, ppc64.AADDMEVCC, ppc64.AADDMEV, ppc64.AADDME,
		ppc64.AADDZECC, ppc64.AADDZEVCC, ppc64.AADDZEV, ppc64.AADDZE,
		ppc64.ACNTLZDCC, ppc64.ACNTLZD, ppc64.ACNTLZWCC, ppc64.ACNTLZW,
		ppc64.AEXTSBCC, ppc64.AEXTSB, ppc64.AEXTSHCC, ppc64.AEXTSH,
		ppc64.AEXTSWCC, ppc64.AEXTSW, ppc64.ANEGCC, ppc64.ANEGVCC,
		ppc64.ANEGV, ppc64.ANEG, ppc64.ASLBMFEE, ppc64.ASLBMFEV,
		ppc64.ASLBMTE, ppc64.ASUBMECC, ppc64.ASUBMEVCC, ppc64.ASUBMEV,
		ppc64.ASUBME, ppc64.ASUBZECC, ppc64.ASUBZEVCC, ppc64.ASUBZEV,
		ppc64.ASUBZE:
		return true
	}
	return false
}

func ppc64RegisterNumber(name string, n int16) (int16, bool) {
	switch name {
	case "CR":
		if 0 <= n && n <= 7 {
			return ppc64.REG_CR0 + n, true
		}
	case "A":
		if 0 <= n && n <= 8 {
			return ppc64.REG_A0 + n, true
		}
	case "VS":
		if 0 <= n && n <= 63 {
			return ppc64.REG_VS0 + n, true
		}
	case "V":
		if 0 <= n && n <= 31 {
			return ppc64.REG_V0 + n, true
		}
	case "F":
		if 0 <= n && n <= 31 {
			return ppc64.REG_F0 + n, true
		}
	case "R":
		if 0 <= n && n <= 31 {
			return ppc64.REG_R0 + n, true
		}
	case "SPR":
		if 0 <= n && n <= 1024 {
			return ppc64.REG_SPR0 + n, true
		}
	}
	return 0, false
}

```

// === FILE: references!/go/src/cmd/asm/internal/arch/riscv64.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file encapsulates some of the odd characteristics of the RISCV64
// instruction set, to minimize its interaction with the core of the
// assembler.

package arch

import (
	"cmd/internal/obj"
	"cmd/internal/obj/riscv"
	"fmt"
)

// IsRISCV64AMO reports whether op is an AMO instruction that requires
// special handling.
func IsRISCV64AMO(op obj.As) bool {
	switch op {
	case riscv.ASCW, riscv.ASCD, riscv.AAMOSWAPW, riscv.AAMOSWAPD, riscv.AAMOADDW, riscv.AAMOADDD,
		riscv.AAMOANDW, riscv.AAMOANDD, riscv.AAMOORW, riscv.AAMOORD, riscv.AAMOXORW, riscv.AAMOXORD,
		riscv.AAMOMINW, riscv.AAMOMIND, riscv.AAMOMINUW, riscv.AAMOMINUD,
		riscv.AAMOMAXW, riscv.AAMOMAXD, riscv.AAMOMAXUW, riscv.AAMOMAXUD:
		return true
	}
	return false
}

// IsRISCV64VTypeI reports whether op is a vtype immediate instruction that
// requires special handling.
func IsRISCV64VTypeI(op obj.As) bool {
	return op == riscv.AVSETVLI || op == riscv.AVSETIVLI
}

// IsRISCV64CSRO reports whether the op is an instruction that uses
// CSR symbolic names and whether that instruction expects a register
// or an immediate source operand.
func IsRISCV64CSRO(op obj.As) (imm bool, ok bool) {
	switch op {
	case riscv.ACSRRCI, riscv.ACSRRSI, riscv.ACSRRWI:
		imm = true
		fallthrough
	case riscv.ACSRRC, riscv.ACSRRS, riscv.ACSRRW:
		ok = true
	}
	return
}

// IsRISCV64PseudoCSRO reports whether the op is a pseudo instruction
// that uses CSR symbolic names, whether that instruction expects a register
// or an immediate source operand and what the expected index of the operand
// containing the CSR name should be.
func IsRISCV64PseudoCSRO(op obj.As) (imm bool, index int, ok bool) {
	switch op {
	case riscv.ACSRCI, riscv.ACSRSI, riscv.ACSRWI:
		imm, index, ok = true, 1, true
	case riscv.ACSRC, riscv.ACSRS, riscv.ACSRW:
		imm, index, ok = false, 1, true
	case riscv.ACSRR:
		imm, index, ok = false, 0, true
	}
	return
}

var riscv64SpecialOperand map[string]riscv.SpecialOperand

// RISCV64SpecialOperand returns the internal representation of a special operand.
func RISCV64SpecialOperand(name string) riscv.SpecialOperand {
	if riscv64SpecialOperand == nil {
		// Generate mapping when function is first called.
		riscv64SpecialOperand = map[string]riscv.SpecialOperand{}
		for opd := riscv.SPOP_RVV_BEGIN; opd < riscv.SPOP_RVV_END; opd++ {
			riscv64SpecialOperand[opd.String()] = opd
		}
		// Add the CSRs
		for csrCode, csrName := range riscv.CSRs {
			// The set of RVV special operand names and the set of CSR special operands
			// names are disjoint and so can safely share a single namespace. However,
			// it's possible that a future update to the CSRs in inst.go could introduce
			// a conflict. This check ensures that such a conflict does not go
			// unnoticed.
			if _, ok := riscv64SpecialOperand[csrName]; ok {
				panic(fmt.Sprintf("riscv64 special operand %q redefined", csrName))
			}
			riscv64SpecialOperand[csrName] = riscv.SpecialOperand(int(csrCode) + int(riscv.SPOP_CSR_BEGIN))
		}
		// Add the FENCE operands
		for opd := riscv.SPOP_FENCE_BEGIN + 1; opd < riscv.SPOP_FENCE_END; opd++ {
			riscv64SpecialOperand[opd.String()] = opd
		}
	}
	if opd, ok := riscv64SpecialOperand[name]; ok {
		return opd
	}
	return riscv.SPOP_END
}

// RISCV64ValidateVectorType reports whether the given configuration is a
// valid vector type.
func RISCV64ValidateVectorType(vsew, vlmul, vtail, vmask int64) error {
	_, err := riscv.EncodeVectorType(vsew, vlmul, vtail, vmask)
	return err
}

```

// === FILE: references!/go/src/cmd/asm/internal/arch/s390x.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file encapsulates some of the odd characteristics of the
// s390x instruction set, to minimize its interaction
// with the core of the assembler.

package arch

import (
	"cmd/internal/obj/s390x"
)

func jumpS390x(word string) bool {
	switch word {
	case "BRC",
		"BC",
		"BCL",
		"BEQ",
		"BGE",
		"BGT",
		"BL",
		"BLE",
		"BLEU",
		"BLT",
		"BLTU",
		"BNE",
		"BR",
		"BVC",
		"BVS",
		"BRCT",
		"BRCTG",
		"CMPBEQ",
		"CMPBGE",
		"CMPBGT",
		"CMPBLE",
		"CMPBLT",
		"CMPBNE",
		"CMPUBEQ",
		"CMPUBGE",
		"CMPUBGT",
		"CMPUBLE",
		"CMPUBLT",
		"CMPUBNE",
		"CRJ",
		"CGRJ",
		"CLRJ",
		"CLGRJ",
		"CIJ",
		"CGIJ",
		"CLIJ",
		"CLGIJ",
		"CALL",
		"JMP":
		return true
	}
	return false
}

func s390xRegisterNumber(name string, n int16) (int16, bool) {
	switch name {
	case "AR":
		if 0 <= n && n <= 15 {
			return s390x.REG_AR0 + n, true
		}
	case "F":
		if 0 <= n && n <= 15 {
			return s390x.REG_F0 + n, true
		}
	case "R":
		if 0 <= n && n <= 15 {
			return s390x.REG_R0 + n, true
		}
	case "V":
		if 0 <= n && n <= 31 {
			return s390x.REG_V0 + n, true
		}
	}
	return 0, false
}

```

// === FILE: references!/go/src/cmd/asm/internal/asm/asm.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asm

import (
	"fmt"
	"internal/abi"
	"strconv"
	"strings"
	"text/scanner"

	"cmd/asm/internal/arch"
	"cmd/asm/internal/flags"
	"cmd/asm/internal/lex"
	"cmd/internal/obj"
	"cmd/internal/obj/arm64"
	"cmd/internal/obj/ppc64"
	"cmd/internal/obj/riscv"
	"cmd/internal/obj/x86"
	"cmd/internal/sys"
)

// TODO: configure the architecture

var testOut *strings.Builder // Gathers output when testing.

// append adds the Prog to the end of the program-thus-far.
// If doLabel is set, it also defines the labels collect for this Prog.
func (p *Parser) append(prog *obj.Prog, cond string, doLabel bool) {
	if cond != "" {
		switch p.arch.Family {
		case sys.ARM:
			if !arch.ARMConditionCodes(prog, cond) {
				p.errorf("unrecognized condition code .%q", cond)
				return
			}

		case sys.ARM64:
			if !arch.ARM64Suffix(prog, cond) {
				p.errorf("unrecognized suffix .%q", cond)
				return
			}

		case sys.AMD64, sys.I386:
			if err := x86.ParseSuffix(prog, cond); err != nil {
				p.errorf("%v", err)
				return
			}
		case sys.RISCV64:
			if err := riscv.ParseSuffix(prog, cond); err != nil {
				p.errorf("unrecognized suffix .%q", cond)
				return
			}
		default:
			p.errorf("unrecognized suffix .%q", cond)
			return
		}
	}
	if p.firstProg == nil {
		p.firstProg = prog
	} else {
		p.lastProg.Link = prog
	}
	p.lastProg = prog
	if doLabel {
		p.pc++
		for _, label := range p.pendingLabels {
			if p.labels[label] != nil {
				p.errorf("label %q multiply defined", label)
				return
			}
			p.labels[label] = prog
		}
		p.pendingLabels = p.pendingLabels[0:0]
	}
	prog.Pc = p.pc
	if *flags.Debug {
		fmt.Println(p.lineNum, prog)
	}
	if testOut != nil {
		fmt.Fprintln(testOut, prog)
	}
}

// validSymbol checks that addr represents a valid name for a pseudo-op.
func (p *Parser) validSymbol(pseudo string, addr *obj.Addr, offsetOk bool) bool {
	if addr.Sym == nil || addr.Name != obj.NAME_EXTERN && addr.Name != obj.NAME_STATIC || addr.Scale != 0 || addr.Reg != 0 {
		p.errorf("%s symbol %q must be a symbol(SB)", pseudo, symbolName(addr))
		return false
	}
	if !offsetOk && addr.Offset != 0 {
		p.errorf("%s symbol %q must not be offset from SB", pseudo, symbolName(addr))
		return false
	}
	return true
}

// evalInteger evaluates an integer constant for a pseudo-op.
func (p *Parser) evalInteger(pseudo string, operands []lex.Token) int64 {
	addr := p.address(operands)
	return p.getConstantPseudo(pseudo, &addr)
}

// validImmediate checks that addr represents an immediate constant.
func (p *Parser) validImmediate(pseudo string, addr *obj.Addr) bool {
	if addr.Type != obj.TYPE_CONST || addr.Name != 0 || addr.Reg != 0 || addr.Index != 0 {
		p.errorf("%s: expected immediate constant; found %s", pseudo, obj.Dconv(&emptyProg, addr))
		return false
	}
	return true
}

// asmText assembles a TEXT pseudo-op.
// TEXT runtime·sigtramp(SB),4,$0-0
func (p *Parser) asmText(operands [][]lex.Token) {
	if len(operands) != 2 && len(operands) != 3 {
		p.errorf("expect two or three operands for TEXT")
		return
	}

	// Labels are function scoped. Patch existing labels and
	// create a new label space for this TEXT.
	p.patch()
	p.labels = make(map[string]*obj.Prog)

	// Operand 0 is the symbol name in the form foo(SB).
	// That means symbol plus indirect on SB and no offset.
	nameAddr := p.address(operands[0])
	if !p.validSymbol("TEXT", &nameAddr, false) {
		return
	}
	name := symbolName(&nameAddr)
	next := 1

	// Next operand is the optional text flag, a literal integer.
	var flag = int64(0)
	if len(operands) == 3 {
		flag = p.evalInteger("TEXT", operands[1])
		next++
	}

	// Issue an error if we see a function defined as ABIInternal
	// without NOSPLIT. In ABIInternal, obj needs to know the function
	// signature in order to construct the morestack path, so this
	// currently isn't supported for asm functions.
	if nameAddr.Sym.ABI() == obj.ABIInternal && flag&obj.NOSPLIT == 0 {
		p.errorf("TEXT %q: ABIInternal requires NOSPLIT", name)
	}

	// Next operand is the frame and arg size.
	// Bizarre syntax: $frameSize-argSize is two words, not subtraction.
	// Both frameSize and argSize must be simple integers; only frameSize
	// can be negative.
	// The "-argSize" may be missing; if so, set it to objabi.ArgsSizeUnknown.
	// Parse left to right.
	op := operands[next]
	if len(op) < 2 || op[0].ScanToken != '$' {
		p.errorf("TEXT %s: frame size must be an immediate constant", name)
		return
	}
	op = op[1:]
	negative := false
	if op[0].ScanToken == '-' {
		negative = true
		op = op[1:]
	}
	if len(op) == 0 || op[0].ScanToken != scanner.Int {
		p.errorf("TEXT %s: frame size must be an immediate constant", name)
		return
	}
	frameSize := p.positiveAtoi(op[0].String())
	if negative {
		frameSize = -frameSize
	}
	op = op[1:]
	argSize := int64(abi.ArgsSizeUnknown)
	if len(op) > 0 {
		// There is an argument size. It must be a minus sign followed by a non-negative integer literal.
		if len(op) != 2 || op[0].ScanToken != '-' || op[1].ScanToken != scanner.Int {
			p.errorf("TEXT %s: argument size must be of form -integer", name)
			return
		}
		argSize = p.positiveAtoi(op[1].String())
	}
	p.ctxt.InitTextSym(nameAddr.Sym, int(flag), p.pos())
	prog := &obj.Prog{
		Ctxt: p.ctxt,
		As:   obj.ATEXT,
		Pos:  p.pos(),
		From: nameAddr,
		To: obj.Addr{
			Type:   obj.TYPE_TEXTSIZE,
			Offset: frameSize,
			// Argsize set below.
		},
	}
	nameAddr.Sym.Func().Text = prog
	prog.To.Val = int32(argSize)
	p.append(prog, "", true)
}

// asmData assembles a DATA pseudo-op.
// DATA masks<>+0x00(SB)/4, $0x00000000
func (p *Parser) asmData(operands [][]lex.Token) {
	if len(operands) != 2 {
		p.errorf("expect two operands for DATA")
		return
	}

	// Operand 0 has the general form foo<>+0x04(SB)/4.
	op := operands[0]
	n := len(op)
	if n < 3 || op[n-2].ScanToken != '/' || op[n-1].ScanToken != scanner.Int {
		p.errorf("expect /size for DATA argument")
		return
	}
	szop := op[n-1].String()
	sz, err := strconv.Atoi(szop)
	if err != nil {
		p.errorf("bad size for DATA argument: %q", szop)
	}
	op = op[:n-2]
	nameAddr := p.address(op)
	if !p.validSymbol("DATA", &nameAddr, true) {
		return
	}
	name := symbolName(&nameAddr)

	// Operand 1 is an immediate constant or address.
	valueAddr := p.address(operands[1])
	switch valueAddr.Type {
	case obj.TYPE_CONST, obj.TYPE_FCONST, obj.TYPE_SCONST, obj.TYPE_ADDR:
		// OK
	default:
		p.errorf("DATA value must be an immediate constant or address")
		return
	}

	// The addresses must not overlap. Easiest test: require monotonicity.
	if lastAddr, ok := p.dataAddr[name]; ok && nameAddr.Offset < lastAddr {
		p.errorf("overlapping DATA entry for %s", name)
		return
	}
	p.dataAddr[name] = nameAddr.Offset + int64(sz)

	switch valueAddr.Type {
	case obj.TYPE_CONST:
		switch sz {
		case 1, 2, 4, 8:
			nameAddr.Sym.WriteInt(p.ctxt, nameAddr.Offset, sz, valueAddr.Offset)
		default:
			p.errorf("bad int size for DATA argument: %d", sz)
		}
	case obj.TYPE_FCONST:
		switch sz {
		case 4:
			nameAddr.Sym.WriteFloat32(p.ctxt, nameAddr.Offset, float32(valueAddr.Val.(float64)))
		case 8:
			nameAddr.Sym.WriteFloat64(p.ctxt, nameAddr.Offset, valueAddr.Val.(float64))
		default:
			p.errorf("bad float size for DATA argument: %d", sz)
		}
	case obj.TYPE_SCONST:
		nameAddr.Sym.WriteString(p.ctxt, nameAddr.Offset, sz, valueAddr.Val.(string))
	case obj.TYPE_ADDR:
		if sz == p.arch.PtrSize {
			nameAddr.Sym.WriteAddr(p.ctxt, nameAddr.Offset, sz, valueAddr.Sym, valueAddr.Offset)
		} else {
			p.errorf("bad addr size for DATA argument: %d", sz)
		}
	}
}

// asmGlobl assembles a GLOBL pseudo-op.
// GLOBL shifts<>(SB),8,$256
// GLOBL shifts<>(SB),$256
func (p *Parser) asmGlobl(operands [][]lex.Token) {
	if len(operands) != 2 && len(operands) != 3 {
		p.errorf("expect two or three operands for GLOBL")
		return
	}

	// Operand 0 has the general form foo<>+0x04(SB).
	nameAddr := p.address(operands[0])
	if !p.validSymbol("GLOBL", &nameAddr, false) {
		return
	}
	next := 1

	// Next operand is the optional flag, a literal integer.
	var flag = int64(0)
	if len(operands) == 3 {
		flag = p.evalInteger("GLOBL", operands[1])
		next++
	}

	// Final operand is an immediate constant.
	addr := p.address(operands[next])
	if !p.validImmediate("GLOBL", &addr) {
		return
	}

	// log.Printf("GLOBL %s %d, $%d", name, flag, size)
	p.ctxt.GloblPos(nameAddr.Sym, addr.Offset, int(flag), p.pos())
}

// asmPCData assembles a PCDATA pseudo-op.
// PCDATA $2, $705
func (p *Parser) asmPCData(operands [][]lex.Token) {
	if len(operands) != 2 {
		p.errorf("expect two operands for PCDATA")
		return
	}

	// Operand 0 must be an immediate constant.
	key := p.address(operands[0])
	if !p.validImmediate("PCDATA", &key) {
		return
	}

	// Operand 1 must be an immediate constant.
	value := p.address(operands[1])
	if !p.validImmediate("PCDATA", &value) {
		return
	}

	// log.Printf("PCDATA $%d, $%d", key.Offset, value.Offset)
	prog := &obj.Prog{
		Ctxt: p.ctxt,
		As:   obj.APCDATA,
		Pos:  p.pos(),
		From: key,
		To:   value,
	}
	p.append(prog, "", true)
}

// asmPCAlign assembles a PCALIGN pseudo-op.
// PCALIGN $16
func (p *Parser) asmPCAlign(operands [][]lex.Token) {
	if len(operands) != 1 {
		p.errorf("expect one operand for PCALIGN")
		return
	}

	// Operand 0 must be an immediate constant.
	key := p.address(operands[0])
	if !p.validImmediate("PCALIGN", &key) {
		return
	}

	prog := &obj.Prog{
		Ctxt: p.ctxt,
		As:   obj.APCALIGN,
		Pos:  p.pos(),
		From: key,
	}
	p.append(prog, "", true)
}

// asmFuncData assembles a FUNCDATA pseudo-op.
// FUNCDATA $1, funcdata<>+4(SB)
func (p *Parser) asmFuncData(operands [][]lex.Token) {
	if len(operands) != 2 {
		p.errorf("expect two operands for FUNCDATA")
		return
	}

	// Operand 0 must be an immediate constant.
	valueAddr := p.address(operands[0])
	if !p.validImmediate("FUNCDATA", &valueAddr) {
		return
	}

	// Operand 1 is a symbol name in the form foo(SB).
	nameAddr := p.address(operands[1])
	if !p.validSymbol("FUNCDATA", &nameAddr, true) {
		return
	}

	prog := &obj.Prog{
		Ctxt: p.ctxt,
		As:   obj.AFUNCDATA,
		Pos:  p.pos(),
		From: valueAddr,
		To:   nameAddr,
	}
	p.append(prog, "", true)
}

// asmJump assembles a jump instruction.
// JMP	R1
// JMP	exit
// JMP	3(PC)
func (p *Parser) asmJump(op obj.As, cond string, a []obj.Addr) {
	var target *obj.Addr
	prog := &obj.Prog{
		Ctxt: p.ctxt,
		Pos:  p.pos(),
		As:   op,
	}
	targetAddr := &prog.To
	switch len(a) {
	case 0:
		if p.arch.Family == sys.Wasm {
			target = &obj.Addr{Type: obj.TYPE_NONE}
			break
		}
		p.errorf("wrong number of arguments to %s instruction", op)
		return
	case 1:
		target = &a[0]
	case 2:
		// Special 2-operand jumps.
		if p.arch.Family == sys.ARM64 && arch.IsARM64ADR(op) {
			// ADR label, R. Label is in From.
			target = &a[0]
			prog.To = a[1]
			targetAddr = &prog.From
		} else {
			target = &a[1]
			prog.From = a[0]
		}
	case 3:
		if p.arch.Family == sys.PPC64 {
			// Special 3-operand jumps.
			// a[1] is a register number expressed as a constant or register value
			target = &a[2]
			prog.From = a[0]
			if a[0].Type != obj.TYPE_CONST {
				// Legacy code may use a plain constant, accept it, and coerce
				// into a constant. E.g:
				//   BC 4,...
				// into
				//   BC $4,...
				prog.From = obj.Addr{
					Type:   obj.TYPE_CONST,
					Offset: p.getConstant(prog, op, &a[0]),
				}

			}

			// Likewise, fixup usage like:
			//   BC x,LT,...
			//   BC x,foo+2,...
			//   BC x,4
			//   BC x,$5
			// into
			//   BC x,CR0LT,...
			//   BC x,CR0EQ,...
			//   BC x,CR1LT,...
			//   BC x,CR1GT,...
			// The first and second cases demonstrate a symbol name which is
			// effectively discarded. In these cases, the offset determines
			// the CR bit.
			prog.Reg = a[1].Reg
			if a[1].Type != obj.TYPE_REG {
				// The CR bit is represented as a constant 0-31. Convert it to a Reg.
				c := p.getConstant(prog, op, &a[1])
				reg, success := ppc64.ConstantToCRbit(c)
				if !success {
					p.errorf("invalid CR bit register number %d", c)
				}
				prog.Reg = reg
			}
			break
		}
		if p.arch.Family == sys.MIPS || p.arch.Family == sys.MIPS64 || p.arch.Family == sys.RISCV64 {
			// 3-operand jumps.
			// First two must be registers
			target = &a[2]
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			break
		}
		if p.arch.Family == sys.Loong64 {
			// 3-operand jumps.
			// First two must be registers
			target = &a[2]
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			break
		}
		if p.arch.Family == sys.S390X {
			// 3-operand jumps.
			target = &a[2]
			prog.From = a[0]
			if a[1].Reg != 0 {
				// Compare two registers and jump.
				prog.Reg = p.getRegister(prog, op, &a[1])
			} else {
				// Compare register with immediate and jump.
				prog.AddRestSource(a[1])
			}
			break
		}
		if p.arch.Family == sys.ARM64 {
			// Special 3-operand jumps.
			// a[0] must be immediate constant; a[1] is a register.
			if a[0].Type != obj.TYPE_CONST {
				p.errorf("%s: expected immediate constant; found %s", op, obj.Dconv(prog, &a[0]))
				return
			}
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			target = &a[2]
			break
		}
		p.errorf("wrong number of arguments to %s instruction", op)
		return
	case 4:
		if p.arch.Family == sys.S390X || p.arch.Family == sys.PPC64 {
			// 4-operand compare-and-branch.
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.AddRestSource(a[2])
			target = &a[3]
			break
		}
		p.errorf("wrong number of arguments to %s instruction", op)
		return
	default:
		p.errorf("wrong number of arguments to %s instruction", op)
		return
	}
	switch {
	case target.Type == obj.TYPE_BRANCH:
		// JMP 4(PC)
		*targetAddr = obj.Addr{
			Type:   obj.TYPE_BRANCH,
			Offset: p.pc + 1 + target.Offset, // +1 because p.pc is incremented in append, below.
		}
	case target.Type == obj.TYPE_REG:
		// JMP R1
		*targetAddr = *target
	case target.Type == obj.TYPE_MEM && (target.Name == obj.NAME_EXTERN || target.Name == obj.NAME_STATIC):
		// JMP main·morestack(SB)
		*targetAddr = *target
	case target.Type == obj.TYPE_INDIR && (target.Name == obj.NAME_EXTERN || target.Name == obj.NAME_STATIC):
		// JMP *main·morestack(SB)
		*targetAddr = *target
		targetAddr.Type = obj.TYPE_INDIR
	case target.Type == obj.TYPE_MEM && target.Reg == 0 && target.Offset == 0:
		// JMP exit
		if target.Sym == nil {
			// Parse error left name unset.
			return
		}
		targetProg := p.labels[target.Sym.Name]
		if targetProg == nil {
			p.toPatch = append(p.toPatch, Patch{targetAddr, target.Sym.Name})
		} else {
			p.branch(targetAddr, targetProg)
		}
	case target.Type == obj.TYPE_MEM && target.Name == obj.NAME_NONE:
		// JMP 4(R0)
		*targetAddr = *target
		// On the ppc64, 9a encodes BR (CTR) as BR CTR. We do the same.
		if p.arch.Family == sys.PPC64 && target.Offset == 0 {
			targetAddr.Type = obj.TYPE_REG
		}
	case target.Type == obj.TYPE_CONST:
		// JMP $4
		*targetAddr = a[0]
	case target.Type == obj.TYPE_NONE:
		// JMP
	default:
		p.errorf("cannot assemble jump %+v", target)
		return
	}

	p.append(prog, cond, true)
}

func (p *Parser) patch() {
	for _, patch := range p.toPatch {
		targetProg := p.labels[patch.label]
		if targetProg == nil {
			p.errorf("undefined label %s", patch.label)
			return
		}
		p.branch(patch.addr, targetProg)
	}
	p.toPatch = p.toPatch[:0]
}

func (p *Parser) branch(addr *obj.Addr, target *obj.Prog) {
	*addr = obj.Addr{
		Type:  obj.TYPE_BRANCH,
		Index: 0,
	}
	addr.Val = target
}

func isARM64SVE(op obj.As) bool {
	return op > arm64.ASVESTART
}

// asmInstruction assembles an instruction.
// MOVW R9, (R10)
func (p *Parser) asmInstruction(op obj.As, cond string, a []obj.Addr) {
	// fmt.Printf("%s %+v\n", op, a)
	prog := &obj.Prog{
		Ctxt: p.ctxt,
		Pos:  p.pos(),
		As:   op,
	}
	switch len(a) {
	case 0:
		// Nothing to do.
	case 1:
		if p.arch.UnaryDst[op] || op == obj.ARET || op == obj.AGETCALLERPC {
			// prog.From is no address.
			prog.To = a[0]
		} else {
			prog.From = a[0]
			// prog.To is no address.
		}
		if p.arch.Family == sys.PPC64 && arch.IsPPC64NEG(op) {
			// NEG: From and To are both a[0].
			prog.To = a[0]
			prog.From = a[0]
			break
		}
	case 2:
		if p.arch.Family == sys.ARM {
			if arch.IsARMCMP(op) {
				prog.From = a[0]
				prog.Reg = p.getRegister(prog, op, &a[1])
				break
			}
			// Strange special cases.
			if arch.IsARMFloatCmp(op) {
				prog.From = a[0]
				prog.Reg = p.getRegister(prog, op, &a[1])
				break
			}
		} else if p.arch.Family == sys.ARM64 && arch.IsARM64CMP(op) {
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			break
		} else if p.arch.Family == sys.MIPS || p.arch.Family == sys.MIPS64 {
			if arch.IsMIPSCMP(op) || arch.IsMIPSMUL(op) {
				prog.From = a[0]
				prog.Reg = p.getRegister(prog, op, &a[1])
				break
			}
		} else if p.arch.Family == sys.Loong64 {
			if arch.IsLoong64RDTIME(op) {
				// The Loong64 RDTIME family of instructions is a bit special,
				// in that both its register operands are outputs
				prog.To = a[0]
				if a[1].Type != obj.TYPE_REG {
					p.errorf("invalid addressing modes for 2nd operand to %s instruction, must be register", op)
					return
				}
				prog.RegTo2 = a[1].Reg
				break
			}

			if arch.IsLoong64PRELD(op) {
				prog.From = a[0]
				prog.AddRestSource(a[1])
				break
			}
		} else if p.arch.Family == sys.RISCV64 {
			// RISCV64 pseudo instructions that reference CSRs with symbolic names.
			if isImm, specialIndex, ok := arch.IsRISCV64PseudoCSRO(op); ok {
				if a[0].Type != obj.TYPE_CONST && isImm {
					p.errorf("invalid value for first operand to %s instruction, must be a 5 bit unsigned immediate", op)
					return
				}
				if a[specialIndex].Type != obj.TYPE_SPECIAL {
					p.errorf("invalid value for first or second operand to %s instruction, must be a CSR name", op)
					return
				}
				prog.AddRestSourceArgs([]obj.Addr{a[specialIndex]})
				if specialIndex == 0 {
					prog.To = a[1]
				} else {
					prog.From = a[0]
				}
				break
			}
		}
		prog.From = a[0]
		prog.To = a[1]
	case 3:
		switch p.arch.Family {
		case sys.MIPS, sys.MIPS64:
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.To = a[2]
		case sys.Loong64:
			switch {
			// Loong64 atomic instructions with one input and two outputs.
			case arch.IsLoong64AMO(op):
				prog.From = a[0]
				prog.To = a[1]
				prog.RegTo2 = a[2].Reg

			case arch.IsLoong64PRELD(op):
				prog.From = a[0]
				prog.AddRestSourceArgs([]obj.Addr{a[1], a[2]})

			default:
				prog.From = a[0]
				prog.Reg = p.getRegister(prog, op, &a[1])
				prog.To = a[2]
			}
		case sys.ARM:
			// Special cases.
			if arch.IsARMSTREX(op) {
				/*
					STREX x, (y), z
						from=(y) reg=x to=z
				*/
				prog.From = a[1]
				prog.Reg = p.getRegister(prog, op, &a[0])
				prog.To = a[2]
				break
			}
			if arch.IsARMBFX(op) {
				// a[0] and a[1] must be constants, a[2] must be a register
				prog.From = a[0]
				prog.AddRestSource(a[1])
				prog.To = a[2]
				break
			}
			// Otherwise the 2nd operand (a[1]) must be a register.
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.To = a[2]
		case sys.AMD64:
			prog.From = a[0]
			prog.AddRestSource(a[1])
			prog.To = a[2]
		case sys.ARM64:
			switch {
			case arch.IsARM64STLXR(op):
				// ARM64 instructions with one input and two outputs.
				prog.From = a[0]
				prog.To = a[1]
				if a[2].Type != obj.TYPE_REG {
					p.errorf("invalid addressing modes for third operand to %s instruction, must be register", op)
					return
				}
				prog.RegTo2 = a[2].Reg
			case arch.IsARM64TBL(op):
				// one of its inputs does not fit into prog.Reg.
				prog.From = a[0]
				prog.AddRestSource(a[1])
				prog.To = a[2]
			case arch.IsARM64CASP(op):
				prog.From = a[0]
				prog.To = a[1]
				// both 1st operand and 3rd operand are (Rs, Rs+1) register pair.
				// And the register pair must be contiguous.
				if (a[0].Type != obj.TYPE_REGREG) || (a[2].Type != obj.TYPE_REGREG) {
					p.errorf("invalid addressing modes for 1st or 3rd operand to %s instruction, must be register pair", op)
					return
				}
				// For ARM64 CASP-like instructions, its 2nd destination operand is register pair(Rt, Rt+1) that can
				// not fit into prog.RegTo2, so save it to the prog.RestArgs.
				prog.AddRestDest(a[2])
			case isARM64SVE(op):
				// SVE instructions, see arm64/goops_gen.go
				prog.From = a[0]
				prog.AddRestSource(a[1])
				prog.To = a[2]
			default:
				prog.From = a[0]
				prog.Reg = p.getRegister(prog, op, &a[1])
				prog.To = a[2]
			}
		case sys.I386:
			prog.From = a[0]
			prog.AddRestSource(a[1])
			prog.To = a[2]
		case sys.PPC64:
			if arch.IsPPC64CMP(op) {
				// CMPW etc.; third argument is a CR register that goes into prog.Reg.
				prog.From = a[0]
				prog.Reg = p.getRegister(prog, op, &a[2])
				prog.To = a[1]
				break
			}

			prog.From = a[0]
			prog.To = a[2]

			// If the second argument is not a register argument, it must be
			// passed RestArgs/AddRestSource
			switch a[1].Type {
			case obj.TYPE_REG:
				prog.Reg = p.getRegister(prog, op, &a[1])
			default:
				prog.AddRestSource(a[1])
			}
		case sys.RISCV64:
			// RISCV64 instructions with one input and two outputs.
			if arch.IsRISCV64AMO(op) {
				prog.From = a[0]
				prog.To = a[1]
				if a[2].Type != obj.TYPE_REG {
					p.errorf("invalid addressing modes for third operand to %s instruction, must be register", op)
					return
				}
				prog.RegTo2 = a[2].Reg
				break
			}
			// RISCV64 instructions that reference CSRs with symbolic names.
			if isImm, ok := arch.IsRISCV64CSRO(op); ok {
				if a[0].Type != obj.TYPE_CONST && isImm {
					p.errorf("invalid value for first operand to %s instruction, must be a 5 bit unsigned immediate", op)
					return
				}
				if a[1].Type != obj.TYPE_SPECIAL {
					p.errorf("invalid value for second operand to %s instruction, must be a CSR name", op)
					return
				}
				prog.AddRestSourceArgs([]obj.Addr{a[1]})
				prog.From = a[0]
				prog.To = a[2]
				break
			}
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.To = a[2]
		case sys.S390X:
			prog.From = a[0]
			if a[1].Type == obj.TYPE_REG {
				prog.Reg = p.getRegister(prog, op, &a[1])
			} else {
				prog.AddRestSource(a[1])
			}
			prog.To = a[2]
		default:
			p.errorf("TODO: implement three-operand instructions for this architecture")
			return
		}
	case 4:
		if p.arch.Family == sys.ARM {
			if arch.IsARMBFX(op) {
				// a[0] and a[1] must be constants, a[2] and a[3] must be registers
				prog.From = a[0]
				prog.AddRestSource(a[1])
				prog.Reg = p.getRegister(prog, op, &a[2])
				prog.To = a[3]
				break
			}
			if arch.IsARMMULA(op) {
				// All must be registers.
				p.getRegister(prog, op, &a[0])
				r1 := p.getRegister(prog, op, &a[1])
				r2 := p.getRegister(prog, op, &a[2])
				p.getRegister(prog, op, &a[3])
				prog.From = a[0]
				prog.To = a[3]
				prog.To.Type = obj.TYPE_REGREG2
				prog.To.Offset = int64(r2)
				prog.Reg = r1
				break
			}
		}
		if p.arch.Family == sys.AMD64 {
			prog.From = a[0]
			prog.AddRestSourceArgs([]obj.Addr{a[1], a[2]})
			prog.To = a[3]
			break
		}
		if p.arch.Family == sys.ARM64 {
			if isARM64SVE(op) {
				// SVE instructions, see arm64/goops_gen.go
				prog.From = a[0]
				prog.AddRestSourceArgs([]obj.Addr{a[1], a[2]})
				prog.To = a[3]
				break
			}
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.AddRestSource(a[2])
			prog.To = a[3]
			break
		}
		if p.arch.Family == sys.Loong64 {
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.AddRestSource(a[2])
			prog.To = a[3]
			break
		}
		if p.arch.Family == sys.PPC64 {
			prog.From = a[0]
			prog.To = a[3]
			// If the second argument is not a register argument, it must be
			// passed RestArgs/AddRestSource
			if a[1].Type == obj.TYPE_REG {
				prog.Reg = p.getRegister(prog, op, &a[1])
				prog.AddRestSource(a[2])
			} else {
				// Don't set prog.Reg if a1 isn't a reg arg.
				prog.AddRestSourceArgs([]obj.Addr{a[1], a[2]})
			}
			break
		}
		if p.arch.Family == sys.RISCV64 {
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.AddRestSource(a[2])
			prog.To = a[3]
			break
		}
		if p.arch.Family == sys.S390X {
			if a[1].Type != obj.TYPE_REG {
				p.errorf("second operand must be a register in %s instruction", op)
				return
			}
			prog.From = a[0]
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.AddRestSource(a[2])
			prog.To = a[3]
			break
		}
		p.errorf("can't handle %s instruction with 4 operands", op)
		return
	case 5:
		if p.arch.Family == sys.ARM64 && isARM64SVE(op) {
			// SVE instructions, see arm64/goops_gen.go
			prog.From = a[0]
			prog.AddRestSourceArgs([]obj.Addr{a[1], a[2], a[3]})
			prog.To = a[4]
			break
		}
		if p.arch.Family == sys.PPC64 {
			prog.From = a[0]
			// Second arg is always a register type on ppc64.
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.AddRestSourceArgs([]obj.Addr{a[2], a[3]})
			prog.To = a[4]
			break
		}
		if p.arch.Family == sys.AMD64 {
			prog.From = a[0]
			prog.AddRestSourceArgs([]obj.Addr{a[1], a[2], a[3]})
			prog.To = a[4]
			break
		}
		if p.arch.Family == sys.S390X {
			prog.From = a[0]
			prog.AddRestSourceArgs([]obj.Addr{a[1], a[2], a[3]})
			prog.To = a[4]
			break
		}
		p.errorf("can't handle %s instruction with 5 operands", op)
		return
	case 6:
		if p.arch.Family == sys.ARM64 && isARM64SVE(op) {
			// SVE instructions, see arm64/goops_gen.go
			prog.From = a[0]
			prog.AddRestSourceArgs([]obj.Addr{a[1], a[2], a[3], a[4]})
			prog.To = a[5]
			break
		}
		if p.arch.Family == sys.ARM && arch.IsARMMRC(op) {
			// Strange special case: MCR, MRC.
			prog.To.Type = obj.TYPE_CONST
			x0 := p.getConstant(prog, op, &a[0])
			x1 := p.getConstant(prog, op, &a[1])
			x2 := int64(p.getRegister(prog, op, &a[2]))
			x3 := int64(p.getRegister(prog, op, &a[3]))
			x4 := int64(p.getRegister(prog, op, &a[4]))
			x5 := p.getConstant(prog, op, &a[5])
			// Cond is handled specially for this instruction.
			offset, MRC, ok := arch.ARMMRCOffset(op, cond, x0, x1, x2, x3, x4, x5)
			if !ok {
				p.errorf("unrecognized condition code .%q", cond)
			}
			prog.To.Offset = offset
			cond = ""
			prog.As = MRC // Both instructions are coded as MRC.
			break
		}
		if p.arch.Family == sys.PPC64 {
			prog.From = a[0]
			// Second arg is always a register type on ppc64.
			prog.Reg = p.getRegister(prog, op, &a[1])
			prog.AddRestSourceArgs([]obj.Addr{a[2], a[3], a[4]})
			prog.To = a[5]
			break
		}
		if p.arch.Family == sys.RISCV64 && arch.IsRISCV64VTypeI(op) {
			prog.From = a[0]
			vsew := p.getSpecial(prog, op, &a[1])
			vlmul := p.getSpecial(prog, op, &a[2])
			vtail := p.getSpecial(prog, op, &a[3])
			vmask := p.getSpecial(prog, op, &a[4])
			if err := arch.RISCV64ValidateVectorType(vsew, vlmul, vtail, vmask); err != nil {
				p.errorf("invalid vtype: %v", err)
			}
			prog.AddRestSourceArgs([]obj.Addr{a[1], a[2], a[3], a[4]})
			prog.To = a[5]
			break
		}
		fallthrough
	default:
		p.errorf("can't handle %s instruction with %d operands", op, len(a))
		return
	}

	p.append(prog, cond, true)
}

// symbolName returns the symbol name, or an error string if none is available.
func symbolName(addr *obj.Addr) string {
	if addr.Sym != nil {
		return addr.Sym.Name
	}
	return "<erroneous symbol>"
}

var emptyProg obj.Prog

// getConstantPseudo checks that addr represents a plain constant and returns its value.
func (p *Parser) getConstantPseudo(pseudo string, addr *obj.Addr) int64 {
	if addr.Type != obj.TYPE_MEM || addr.Name != 0 || addr.Reg != 0 || addr.Index != 0 {
		p.errorf("%s: expected integer constant; found %s", pseudo, obj.Dconv(&emptyProg, addr))
	}
	return addr.Offset
}

// getConstant checks that addr represents a plain constant and returns its value.
func (p *Parser) getConstant(prog *obj.Prog, op obj.As, addr *obj.Addr) int64 {
	if addr.Type != obj.TYPE_MEM || addr.Name != 0 || addr.Reg != 0 || addr.Index != 0 {
		p.errorf("%s: expected integer constant; found %s", op, obj.Dconv(prog, addr))
	}
	return addr.Offset
}

// getRegister checks that addr represents a register and returns its value.
func (p *Parser) getRegister(prog *obj.Prog, op obj.As, addr *obj.Addr) int16 {
	if addr.Type != obj.TYPE_REG || addr.Offset != 0 || addr.Name != 0 || addr.Index != 0 {
		p.errorf("%s: expected register; found %s", op, obj.Dconv(prog, addr))
	}
	return addr.Reg
}

// getSpecial checks that addr represents a special operand and returns its value.
func (p *Parser) getSpecial(prog *obj.Prog, op obj.As, addr *obj.Addr) int64 {
	if addr.Type != obj.TYPE_SPECIAL || addr.Name != 0 || addr.Reg != 0 || addr.Index != 0 {
		p.errorf("%s: expected special operand; found %s", op, obj.Dconv(prog, addr))
	}
	return addr.Offset
}

```

// === FILE: references!/go/src/cmd/asm/internal/asm/parse.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package asm implements the parser and instruction generator for the assembler.
// TODO: Split apart?
package asm

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"text/scanner"
	"unicode/utf8"

	"cmd/asm/internal/arch"
	"cmd/asm/internal/flags"
	"cmd/asm/internal/lex"
	"cmd/internal/obj"
	"cmd/internal/obj/arm64"
	"cmd/internal/obj/riscv"
	"cmd/internal/obj/x86"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"cmd/internal/sys"
)

type Parser struct {
	lex           lex.TokenReader
	lineNum       int   // Line number in source file.
	errorLine     int   // Line number of last error.
	errorCount    int   // Number of errors.
	sawCode       bool  // saw code in this file (as opposed to comments and blank lines)
	pc            int64 // virtual PC; count of Progs; doesn't advance for GLOBL or DATA.
	input         []lex.Token
	inputPos      int
	pendingLabels []string // Labels to attach to next instruction.
	labels        map[string]*obj.Prog
	toPatch       []Patch
	addr          []obj.Addr
	arch          *arch.Arch
	ctxt          *obj.Link
	firstProg     *obj.Prog
	lastProg      *obj.Prog
	dataAddr      map[string]int64 // Most recent address for DATA for this symbol.
	isJump        bool             // Instruction being assembled is a jump.
	allowABI      bool             // Whether ABI selectors are allowed.
	pkgPrefix     string           // Prefix to add to local symbols.
	errorWriter   io.Writer
}

type Patch struct {
	addr  *obj.Addr
	label string
}

func NewParser(ctxt *obj.Link, ar *arch.Arch, lexer lex.TokenReader) *Parser {
	pkgPrefix := obj.UnlinkablePkg
	if ctxt != nil {
		pkgPrefix = objabi.PathToPrefix(ctxt.Pkgpath)
	}
	return &Parser{
		ctxt:        ctxt,
		arch:        ar,
		lex:         lexer,
		labels:      make(map[string]*obj.Prog),
		dataAddr:    make(map[string]int64),
		errorWriter: os.Stderr,
		allowABI:    ctxt != nil && objabi.LookupPkgSpecial(ctxt.Pkgpath).AllowAsmABI,
		pkgPrefix:   pkgPrefix,
	}
}

// panicOnError is enabled when testing to abort execution on the first error
// and turn it into a recoverable panic.
var panicOnError bool

func (p *Parser) errorf(format string, args ...any) {
	if panicOnError {
		panic(fmt.Errorf(format, args...))
	}
	if p.lineNum == p.errorLine {
		// Only one error per line.
		return
	}
	p.errorLine = p.lineNum
	if p.lex != nil {
		// Put file and line information on head of message.
		format = "%s:%d: " + format + "\n"
		args = append([]any{p.lex.File(), p.lineNum}, args...)
	}
	fmt.Fprintf(p.errorWriter, format, args...)
	p.errorCount++
	if p.errorCount > 10 && !*flags.AllErrors {
		log.Fatal("too many errors")
	}
}

func (p *Parser) pos() src.XPos {
	return p.ctxt.PosTable.XPos(src.MakePos(p.lex.Base(), uint(p.lineNum), 0))
}

func (p *Parser) Parse() (*obj.Prog, bool) {
	scratch := make([][]lex.Token, 0, 3)
	for {
		word, cond, operands, ok := p.line(scratch)
		if !ok {
			break
		}
		scratch = operands

		if p.pseudo(word, operands) {
			continue
		}
		i, present := p.arch.Instructions[word]
		if present {
			p.instruction(i, word, cond, operands)
			continue
		}
		p.errorf("unrecognized instruction %q", word)
	}
	if p.errorCount > 0 {
		return nil, false
	}
	p.patch()
	return p.firstProg, true
}

// ParseSymABIs parses p's assembly code to find text symbol
// definitions and references and writes a symabis file to w.
func (p *Parser) ParseSymABIs(w io.Writer) bool {
	operands := make([][]lex.Token, 0, 3)
	for {
		word, _, operands1, ok := p.line(operands)
		if !ok {
			break
		}
		operands = operands1

		p.symDefRef(w, word, operands)
	}
	return p.errorCount == 0
}

// nextToken returns the next non-build-comment token from the lexer.
// It reports misplaced //go:build comments but otherwise discards them.
func (p *Parser) nextToken() lex.ScanToken {
	for {
		tok := p.lex.Next()
		if tok == lex.BuildComment {
			if p.sawCode {
				p.errorf("misplaced //go:build comment")
			}
			continue
		}
		if tok != '\n' {
			p.sawCode = true
		}
		if tok == '#' {
			// A leftover wisp of a #include/#define/etc,
			// to let us know that p.sawCode should be true now.
			// Otherwise ignored.
			continue
		}
		return tok
	}
}

// line consumes a single assembly line from p.lex of the form
//
//	{label:} WORD[.cond] [ arg {, arg} ] (';' | '\n')
//
// It adds any labels to p.pendingLabels and returns the word, cond,
// operand list, and true. If there is an error or EOF, it returns
// ok=false.
//
// line may reuse the memory from scratch.
func (p *Parser) line(scratch [][]lex.Token) (word, cond string, operands [][]lex.Token, ok bool) {
next:
	// Skip newlines.
	var tok lex.ScanToken
	for {
		tok = p.nextToken()
		// We save the line number here so error messages from this instruction
		// are labeled with this line. Otherwise we complain after we've absorbed
		// the terminating newline and the line numbers are off by one in errors.
		p.lineNum = p.lex.Line()
		switch tok {
		case '\n', ';':
			continue
		case scanner.EOF:
			return "", "", nil, false
		}
		break
	}
	// First item must be an identifier.
	if tok != scanner.Ident {
		p.errorf("expected identifier, found %q", p.lex.Text())
		return "", "", nil, false // Might as well stop now.
	}
	word, cond = p.lex.Text(), ""
	operands = scratch[:0]
	// Zero or more comma-separated operands, one per loop.
	nesting := 0
	colon := -1
	for tok != '\n' && tok != ';' {
		// Process one operand.
		var items []lex.Token
		if cap(operands) > len(operands) {
			// Reuse scratch items slice.
			items = operands[:cap(operands)][len(operands)][:0]
		} else {
			items = make([]lex.Token, 0, 3)
		}
		for {
			tok = p.nextToken()
			if len(operands) == 0 && len(items) == 0 {
				if p.arch.InFamily(sys.ARM, sys.ARM64, sys.AMD64, sys.I386, sys.Loong64, sys.RISCV64) && tok == '.' {
					// Suffixes: ARM conditionals, Loong64 vector instructions, RISCV rounding mode or x86 modifiers.
					tok = p.nextToken()
					str := p.lex.Text()
					if tok != scanner.Ident {
						p.errorf("instruction suffix expected identifier, found %s", str)
					}
					cond = cond + "." + str
					continue
				}
				if tok == ':' {
					// Labels.
					p.pendingLabels = append(p.pendingLabels, word)
					goto next
				}
			}
			if tok == scanner.EOF {
				p.errorf("unexpected EOF")
				return "", "", nil, false
			}
			// Split operands on comma. Also, the old syntax on x86 for a "register pair"
			// was AX:DX, for which the new syntax is DX, AX. Note the reordering.
			if tok == '\n' || tok == ';' || (nesting == 0 && (tok == ',' || tok == ':')) {
				if tok == ':' {
					// Remember this location so we can swap the operands below.
					if colon >= 0 {
						p.errorf("invalid ':' in operand")
						return word, cond, operands, true
					}
					colon = len(operands)
				}
				break
			}
			if tok == '(' || tok == '[' {
				nesting++
			}
			if tok == ')' || tok == ']' {
				nesting--
			}
			items = append(items, lex.Make(tok, p.lex.Text()))
		}
		if len(items) > 0 {
			operands = append(operands, items)
			if colon >= 0 && len(operands) == colon+2 {
				// AX:DX becomes DX, AX.
				operands[colon], operands[colon+1] = operands[colon+1], operands[colon]
				colon = -1
			}
		} else if len(operands) > 0 || tok == ',' || colon >= 0 {
			// Had a separator with nothing after.
			p.errorf("missing operand")
		}
	}
	return word, cond, operands, true
}

func (p *Parser) instruction(op obj.As, word, cond string, operands [][]lex.Token) {
	p.addr = p.addr[0:0]
	p.isJump = p.arch.IsJump(word)
	for _, op := range operands {
		addr := p.address(op)
		if !p.isJump && addr.Reg < 0 { // Jumps refer to PC, a pseudo.
			p.errorf("illegal use of pseudo-register in %s", word)
		}
		p.addr = append(p.addr, addr)
	}
	if p.isJump {
		p.asmJump(op, cond, p.addr)
		return
	}
	p.asmInstruction(op, cond, p.addr)
}

func (p *Parser) pseudo(word string, operands [][]lex.Token) bool {
	switch word {
	case "DATA":
		p.asmData(operands)
	case "FUNCDATA":
		p.asmFuncData(operands)
	case "GLOBL":
		p.asmGlobl(operands)
	case "PCDATA":
		p.asmPCData(operands)
	case "PCALIGN":
		p.asmPCAlign(operands)
	case "TEXT":
		p.asmText(operands)
	default:
		return false
	}
	return true
}

// symDefRef scans a line for potential text symbol definitions and
// references and writes symabis information to w.
//
// The symabis format is documented at
// cmd/compile/internal/ssagen.ReadSymABIs.
func (p *Parser) symDefRef(w io.Writer, word string, operands [][]lex.Token) {
	switch word {
	case "TEXT":
		// Defines text symbol in operands[0].
		if len(operands) > 0 {
			p.start(operands[0])
			if name, abi, ok := p.funcAddress(); ok {
				fmt.Fprintf(w, "def %s %s\n", name, abi)
			}
		}
		return
	case "GLOBL", "PCDATA":
		// No text definitions or symbol references.
	case "DATA", "FUNCDATA":
		// For DATA, operands[0] is defined symbol.
		// For FUNCDATA, operands[0] is an immediate constant.
		// Remaining operands may have references.
		if len(operands) < 2 {
			return
		}
		operands = operands[1:]
	}
	// Search for symbol references.
	for _, op := range operands {
		p.start(op)
		if name, abi, ok := p.funcAddress(); ok {
			fmt.Fprintf(w, "ref %s %s\n", name, abi)
		}
	}
}

func (p *Parser) start(operand []lex.Token) {
	p.input = operand
	p.inputPos = 0
}

// address parses the operand into a link address structure.
func (p *Parser) address(operand []lex.Token) obj.Addr {
	p.start(operand)
	addr := obj.Addr{}
	p.operand(&addr)
	return addr
}

// parseScale converts a decimal string into a valid scale factor.
func (p *Parser) parseScale(s string) int8 {
	switch s {
	case "1", "2", "4", "8":
		return int8(s[0] - '0')
	}
	p.errorf("bad scale: %s", s)
	return 0
}

// operand parses a general operand and stores the result in *a.
func (p *Parser) operand(a *obj.Addr) {
	//fmt.Printf("Operand: %v\n", p.input)
	if len(p.input) == 0 {
		p.errorf("empty operand: cannot happen")
		return
	}
	// General address (with a few exceptions) looks like
	//	$sym±offset(SB)(reg)(index*scale)
	// Exceptions are:
	//
	//	R1
	//	offset
	//	$offset
	// Every piece is optional, so we scan left to right and what
	// we discover tells us where we are.

	// Prefix: $.
	var prefix rune
	switch tok := p.peek(); tok {
	case '$', '*':
		prefix = rune(tok)
		p.next()
	}

	// Symbol: sym±offset(SB)
	tok := p.next()
	name := tok.String()
	if tok.ScanToken == scanner.Ident && !p.atStartOfRegister(name) {
		// See if this is an architecture specific special operand.
		switch p.arch.Family {
		case sys.ARM64:
			if opd := arch.ARM64SpecialOperand(name); opd != arm64.SPOP_END {
				a.Type = obj.TYPE_SPECIAL
				a.Offset = int64(opd)
			}
		case sys.RISCV64:
			if opd := arch.RISCV64SpecialOperand(name); opd != riscv.SPOP_END {
				a.Type = obj.TYPE_SPECIAL
				a.Offset = int64(opd)
			}
		}

		if a.Type != obj.TYPE_SPECIAL {
			// We have a symbol. Parse $sym±offset(symkind)
			p.symbolReference(a, p.qualifySymbol(name), prefix)
		}
		// fmt.Printf("SYM %s\n", obj.Dconv(&emptyProg, 0, a))
		if p.peek() == scanner.EOF {
			return
		}
	}

	// Detect AC_PREGSEL operand syntax: [selreg, $idximm](preg.T2)
	if tok.ScanToken == '[' && p.arch.Family == sys.ARM64 {
		pos := p.inputPos
		if pos+5 < len(p.input) && p.input[pos].ScanToken == scanner.Ident &&
			p.input[pos+1].ScanToken == ',' && p.input[pos+2].ScanToken == '$' &&
			p.input[pos+3].ScanToken == scanner.Int && p.input[pos+4].ScanToken == ']' &&
			p.input[pos+5].ScanToken == '(' {

			selregStr := p.input[pos].String()
			immStr := p.input[pos+3].String()

			pos += 6 // past '('
			if pos < len(p.input) && p.input[pos].ScanToken == scanner.Ident {
				pregStr := p.input[pos].String()
				pos++
				ext := ""
				if pos < len(p.input) && p.input[pos].ScanToken == '.' {
					pos++
					if pos < len(p.input) && p.input[pos].ScanToken == scanner.Ident {
						ext = p.input[pos].String()
						pos++
					}
				}
				if pos < len(p.input) && p.input[pos].ScanToken == ')' {
					// Matches AC_PREGSEL syntax!
					p.inputPos = pos + 1 // consume the parsed tokens

					// Encode components
					selreg, ok1 := p.registerReference(selregStr)
					preg, ok2 := p.registerReference(pregStr)
					idximm := int64(p.atoi(immStr))

					if !ok1 || !ok2 {
						p.errorf("invalid registers in pregsel operand")
						return
					}

					var arng2 int
					switch ext {
					case "B":
						arng2 = arm64.ARNG_B
					case "H":
						arng2 = arm64.ARNG_H
					case "S":
						arng2 = arm64.ARNG_S
					case "D":
						arng2 = arm64.ARNG_D
					case "Q":
						arng2 = arm64.ARNG_Q
					default:
						p.errorf("invalid P register arrangement: %s", ext)
						return
					}

					// Pack values into a.Offset
					a.Type = obj.TYPE_REG
					a.Offset = int64(preg&31) | int64((arng2&15)<<5) | int64((selreg&31)<<9) | (idximm << 14) | (int64(1) << 62)
					p.expectOperandEnd()
					return
				}
			}
		}
	}

	// Special register list syntax for arm: [R1,R3-R7]
	if tok.ScanToken == '[' {
		if prefix != 0 {
			p.errorf("illegal use of register list")
		}
		p.registerList(a)
		p.expectOperandEnd()
		return
	}

	// Register: R1
	if tok.ScanToken == scanner.Ident && p.atStartOfRegister(name) {
		if p.atRegisterShift() {
			// ARM shifted register such as R1<<R2 or R1>>2.
			a.Type = obj.TYPE_SHIFT
			a.Offset = p.registerShift(tok.String(), prefix)
			if p.peek() == '(' {
				// Can only be a literal register here.
				p.next()
				tok := p.next()
				name := tok.String()
				if !p.atStartOfRegister(name) {
					p.errorf("expected register; found %s", name)
				}
				a.Reg, _ = p.registerReference(name)
				p.get(')')
			}
		} else if p.atRegisterExtension() {
			a.Type = obj.TYPE_REG
			p.registerExtension(a, tok.String(), prefix)
			p.expectOperandEnd()
			return
		} else if r1, r2, scale, ok := p.register(tok.String(), prefix); ok {
			if scale != 0 {
				p.errorf("expected simple register reference")
			}
			a.Type = obj.TYPE_REG
			a.Reg = r1
			if r2 != 0 {
				// Form is R1:R2. It is on RHS and the second register
				// needs to go into the LHS.
				panic("cannot happen (Addr.Reg2)")
			}
		}
		// fmt.Printf("REG %s\n", obj.Dconv(&emptyProg, 0, a))
		p.expectOperandEnd()
		return
	}

	// Detect (VL*imm) or (-VL*imm) pattern in ARM64
	if p.arch.Family == sys.ARM64 && len(p.input) >= 5 && p.input[0].ScanToken == '(' {
		pos := 1
		sign := int64(1)
		if p.input[pos].ScanToken == '-' {
			sign = -1
			pos++
		} else if p.input[pos].ScanToken == '+' {
			pos++
		}
		if pos+3 < len(p.input) && p.input[pos].String() == "VL" && p.input[pos+1].ScanToken == '*' && p.input[pos+2].ScanToken == scanner.Int && p.input[pos+3].ScanToken == ')' {
			imm := int64(p.atoi(p.input[pos+2].String()))
			imm *= sign
			a.Offset = imm
			a.Scale = -32768 // bit 15 signals multiple of Vector Length

			// Remove the (VL*imm) tokens and let operand parse the remaining sequence (reg.T)
			p.input = p.input[pos+4:]
			p.inputPos = 0
			p.operand(a)
			return
		}
	}

	// Constant.
	haveConstant := false
	switch tok.ScanToken {
	case scanner.Int, scanner.Float, scanner.String, scanner.Char, '+', '-', '~':
		haveConstant = true
	case '(':
		// Could be parenthesized expression or (R). Must be something, though.
		tok := p.next()
		if tok.ScanToken == scanner.EOF {
			p.errorf("missing right parenthesis")
			return
		}
		rname := tok.String()
		p.back()
		haveConstant = !p.atStartOfRegister(rname)
		if !haveConstant {
			p.back() // Put back the '('.
		}
	}
	if haveConstant {
		p.back()
		if p.have(scanner.Float) {
			if prefix != '$' {
				p.errorf("floating-point constant must be an immediate")
			}
			a.Type = obj.TYPE_FCONST
			a.Val = p.floatExpr()
			// fmt.Printf("FCONST %s\n", obj.Dconv(&emptyProg, 0, a))
			p.expectOperandEnd()
			return
		}
		if p.have(scanner.String) {
			if prefix != '$' {
				p.errorf("string constant must be an immediate")
				return
			}
			str, err := strconv.Unquote(p.get(scanner.String).String())
			if err != nil {
				p.errorf("string parse error: %s", err)
			}
			a.Type = obj.TYPE_SCONST
			a.Val = str
			// fmt.Printf("SCONST %s\n", obj.Dconv(&emptyProg, 0, a))
			p.expectOperandEnd()
			return
		}
		a.Offset = int64(p.expr())
		if p.peek() != '(' {
			switch prefix {
			case '$':
				a.Type = obj.TYPE_CONST
			case '*':
				a.Type = obj.TYPE_INDIR // Can appear but is illegal, will be rejected by the linker.
			default:
				a.Type = obj.TYPE_MEM
			}
			// fmt.Printf("CONST %d %s\n", a.Offset, obj.Dconv(&emptyProg, 0, a))
			p.expectOperandEnd()
			return
		}
		// fmt.Printf("offset %d \n", a.Offset)
	}

	// Register indirection: (reg) or (index*scale). We are on the opening paren.
	p.registerIndirect(a, prefix)
	// fmt.Printf("DONE %s\n", p.arch.Dconv(&emptyProg, 0, a))

	p.expectOperandEnd()
	return
}

// atStartOfRegister reports whether the parser is at the start of a register definition.
func (p *Parser) atStartOfRegister(name string) bool {
	// Simple register: R10.
	_, present := p.arch.Register[name]
	if present {
		return true
	}
	// Parenthesized register: R(10).
	return p.arch.RegisterPrefix[name] && p.peek() == '('
}

// atRegisterShift reports whether we are at the start of an ARM shifted register.
// We have consumed the register or R prefix.
func (p *Parser) atRegisterShift() bool {
	// ARM only.
	if !p.arch.InFamily(sys.ARM, sys.ARM64) {
		return false
	}
	// R1<<...
	if lex.IsRegisterShift(p.peek()) {
		return true
	}
	// R(1)<<...   Ugly check. TODO: Rethink how we handle ARM register shifts to be
	// less special.
	if p.peek() != '(' || len(p.input)-p.inputPos < 4 {
		return false
	}
	return p.at('(', scanner.Int, ')') && lex.IsRegisterShift(p.input[p.inputPos+3].ScanToken)
}

// atRegisterExtension reports whether we are at the start of an ARM64 extended register.
// We have consumed the register or R prefix.
func (p *Parser) atRegisterExtension() bool {
	switch p.arch.Family {
	case sys.ARM64, sys.Loong64:
		// R1.xxx
		return p.peek() == '.' || p.peek() == '['
	default:
		return false
	}
}

// registerReference parses a register given either the name, R10, or a parenthesized form, SPR(10).
func (p *Parser) registerReference(name string) (int16, bool) {
	r, present := p.arch.Register[name]
	if present {
		return r, true
	}
	if !p.arch.RegisterPrefix[name] {
		p.errorf("expected register; found %s", name)
		return 0, false
	}
	p.get('(')
	tok := p.get(scanner.Int)
	num, err := strconv.ParseInt(tok.String(), 10, 16)
	p.get(')')
	if err != nil {
		p.errorf("parsing register list: %s", err)
		return 0, false
	}
	r, ok := p.arch.RegisterNumber(name, int16(num))
	if !ok {
		p.errorf("illegal register %s(%d)", name, r)
		return 0, false
	}
	return r, true
}

// register parses a full register reference where there is no symbol present (as in 4(R0) or R(10) but not sym(SB))
// including forms involving multiple registers such as R1:R2.
func (p *Parser) register(name string, prefix rune) (r1, r2 int16, scale int8, ok bool) {
	// R1 or R(1) R1:R2 R1,R2 R1+R2, or R1*scale.
	r1, ok = p.registerReference(name)
	if !ok {
		return
	}
	if prefix != 0 && prefix != '*' { // *AX is OK.
		p.errorf("prefix %c not allowed for register: %c%s", prefix, prefix, name)
	}
	c := p.peek()
	if c == ':' || c == ',' || c == '+' {
		// 2nd register; syntax (R1+R2) etc. No two architectures agree.
		// Check the architectures match the syntax.
		switch p.next().ScanToken {
		case ',':
			if !p.arch.InFamily(sys.ARM, sys.ARM64) {
				p.errorf("(register,register) not supported on this architecture")
				return
			}
		case '+':
			if p.arch.Family != sys.PPC64 {
				p.errorf("(register+register) not supported on this architecture")
				return
			}
		}
		name := p.next().String()
		r2, ok = p.registerReference(name)
		if !ok {
			return
		}
	}
	if p.peek() == '*' {
		// Scale
		p.next()
		scale = p.parseScale(p.next().String())
	}
	return r1, r2, scale, true
}

// registerShift parses an ARM/ARM64 shifted register reference and returns the encoded representation.
// There is known to be a register (current token) and a shift operator (peeked token).
func (p *Parser) registerShift(name string, prefix rune) int64 {
	if prefix != 0 {
		p.errorf("prefix %c not allowed for shifted register: $%s", prefix, name)
	}
	// R1 op R2 or r1 op constant.
	// op is:
	//	"<<" == 0
	//	">>" == 1
	//	"->" == 2
	//	"@>" == 3
	r1, ok := p.registerReference(name)
	if !ok {
		return 0
	}
	var op int16
	switch p.next().ScanToken {
	case lex.LSH:
		op = 0
	case lex.RSH:
		op = 1
	case lex.ARR:
		op = 2
	case lex.ROT:
		// following instructions on ARM64 support rotate right
		// AND, ANDS, TST, BIC, BICS, EON, EOR, ORR, MVN, ORN
		op = 3
	}
	tok := p.next()
	str := tok.String()
	var count int16
	switch tok.ScanToken {
	case scanner.Ident:
		if p.arch.Family == sys.ARM64 {
			p.errorf("rhs of shift must be integer: %s", str)
		} else {
			r2, ok := p.registerReference(str)
			if !ok {
				p.errorf("rhs of shift must be register or integer: %s", str)
			}
			count = (r2&15)<<8 | 1<<4
		}
	case scanner.Int, '(':
		p.back()
		x := int64(p.expr())
		if p.arch.Family == sys.ARM64 {
			if x >= 64 {
				p.errorf("register shift count too large: %s", str)
			}
			count = int16((x & 63) << 10)
		} else {
			if x >= 32 {
				p.errorf("register shift count too large: %s", str)
			}
			count = int16((x & 31) << 7)
		}
	default:
		p.errorf("unexpected %s in register shift", tok.String())
	}
	if p.arch.Family == sys.ARM64 {
		off, err := arch.ARM64RegisterShift(r1, op, count)
		if err != nil {
			p.errorf("%v", err)
		}
		return off
	} else {
		return int64((r1 & 15) | op<<5 | count)
	}
}

// registerExtension parses a register with extension or arrangement.
// There is known to be a register (current token) and an extension operator (peeked token).
func (p *Parser) registerExtension(a *obj.Addr, name string, prefix rune) {
	if prefix != 0 {
		p.errorf("prefix %c not allowed for shifted register: $%s", prefix, name)
	}

	reg, ok := p.registerReference(name)
	if !ok {
		p.errorf("unexpected %s in register extension", name)
		return
	}

	isIndex := false
	num := int16(0)
	isAmount := true // Amount is zero by default
	ext := ""
	if p.peek() == lex.LSH {
		// (Rn)(Rm<<2), the shifted offset register.
		ext = "LSL"
	} else if p.peek() == '.' {
		// (Rn)(Rm.UXTW<1), the extended offset register.
		// Rm.UXTW<<3, the extended register.
		p.get('.')
		tok := p.next()
		ext = tok.String()
	}
	if p.peek() == lex.LSH {
		// parses left shift amount applied after extension: <<Amount
		p.get(lex.LSH)
		tok := p.get(scanner.Int)
		amount, err := strconv.ParseInt(tok.String(), 10, 16)
		if err != nil {
			p.errorf("parsing left shift amount: %s", err)
		}
		num = int16(amount)
	} else if p.peek() == '[' {
		// parses an element: [Index]
		p.get('[')
		tok := p.get(scanner.Int)
		index, err := strconv.ParseInt(tok.String(), 10, 16)
		p.get(']')
		if err != nil {
			p.errorf("parsing element index: %s", err)
		}
		isIndex = true
		isAmount = false
		num = int16(index)
	}

	switch p.arch.Family {
	case sys.ARM64:
		err := arm64.EncodeRegisterExtension(a, ext, reg, num, isAmount, isIndex)
		if err != nil {
			p.errorf("%v", err)
		}
	case sys.Loong64:
		err := arch.Loong64RegisterExtension(a, ext, reg, num, isAmount, isIndex)
		if err != nil {
			p.errorf("%v", err)
		}
	default:
		p.errorf("register extension not supported on this architecture")
	}
}

// qualifySymbol returns name as a package-qualified symbol name. If
// name starts with a period, qualifySymbol prepends the package
// prefix. Otherwise it returns name unchanged.
func (p *Parser) qualifySymbol(name string) string {
	if strings.HasPrefix(name, ".") {
		name = p.pkgPrefix + name
	}
	return name
}

// symbolReference parses a symbol that is known not to be a register.
func (p *Parser) symbolReference(a *obj.Addr, name string, prefix rune) {
	// Identifier is a name.
	switch prefix {
	case 0:
		a.Type = obj.TYPE_MEM
	case '$':
		a.Type = obj.TYPE_ADDR
	case '*':
		a.Type = obj.TYPE_INDIR
	}

	// Parse optional <> (indicates a static symbol) or
	// <ABIxxx> (selecting text symbol with specific ABI).
	doIssueError := true
	isStatic, abi := p.symRefAttrs(name, doIssueError)

	if p.peek() == '+' || p.peek() == '-' {
		a.Offset = int64(p.expr())
	}
	if isStatic {
		a.Sym = p.ctxt.LookupStatic(name)
	} else {
		a.Sym = p.ctxt.LookupABI(name, abi)
	}
	if p.peek() == scanner.EOF {
		if prefix == 0 && p.isJump {
			// Symbols without prefix or suffix are jump labels.
			return
		}
		p.errorf("illegal or missing addressing mode for symbol %s", name)
		return
	}
	// Expect (SB), (FP), (PC), or (SP)
	p.get('(')
	reg := p.get(scanner.Ident).String()
	p.get(')')
	p.setPseudoRegister(a, reg, isStatic, prefix)
}

// setPseudoRegister sets the NAME field of addr for a pseudo-register reference such as (SB).
func (p *Parser) setPseudoRegister(addr *obj.Addr, reg string, isStatic bool, prefix rune) {
	if addr.Reg != 0 {
		p.errorf("internal error: reg %s already set in pseudo", reg)
	}
	switch reg {
	case "FP":
		addr.Name = obj.NAME_PARAM
	case "PC":
		if prefix != 0 {
			p.errorf("illegal addressing mode for PC")
		}
		addr.Type = obj.TYPE_BRANCH // We set the type and leave NAME untouched. See asmJump.
	case "SB":
		addr.Name = obj.NAME_EXTERN
		if isStatic {
			addr.Name = obj.NAME_STATIC
		}
	case "SP":
		addr.Name = obj.NAME_AUTO // The pseudo-stack.
	default:
		p.errorf("expected pseudo-register; found %s", reg)
	}
	if prefix == '$' {
		addr.Type = obj.TYPE_ADDR
	}
}

// symRefAttrs parses an optional function symbol attribute clause for
// the function symbol 'name', logging an error for a malformed
// attribute clause if 'issueError' is true. The return value is a
// (boolean, ABI) pair indicating that the named symbol is either
// static or a particular ABI specification.
//
// The expected form of the attribute clause is:
//
// empty,           yielding (false, obj.ABI0)
// "<>",            yielding (true,  obj.ABI0)
// "<ABI0>"         yielding (false, obj.ABI0)
// "<ABIInternal>"  yielding (false, obj.ABIInternal)
//
// Anything else beginning with "<" logs an error if issueError is
// true, otherwise returns (false, obj.ABI0).
func (p *Parser) symRefAttrs(name string, issueError bool) (bool, obj.ABI) {
	abi := obj.ABI0
	isStatic := false
	if p.peek() != '<' {
		return isStatic, abi
	}
	p.next()
	tok := p.peek()
	if tok == '>' {
		isStatic = true
	} else if tok == scanner.Ident {
		abistr := p.get(scanner.Ident).String()
		if !p.allowABI {
			if issueError {
				p.errorf("ABI selector only permitted when compiling runtime, reference was to %q", name)
			}
		} else {
			theabi, valid := obj.ParseABI(abistr)
			if !valid {
				if issueError {
					p.errorf("malformed ABI selector %q in reference to %q",
						abistr, name)
				}
			} else {
				abi = theabi
			}
		}
	}
	p.get('>')
	return isStatic, abi
}

// funcAddress parses an external function address. This is a
// constrained form of the operand syntax that's always SB-based,
// non-static, and has at most a simple integer offset:
//
//	[$|*]sym[<abi>][+Int](SB)
func (p *Parser) funcAddress() (string, obj.ABI, bool) {
	switch p.peek() {
	case '$', '*':
		// Skip prefix.
		p.next()
	}

	tok := p.next()
	name := tok.String()
	if tok.ScanToken != scanner.Ident || p.atStartOfRegister(name) {
		return "", obj.ABI0, false
	}
	name = p.qualifySymbol(name)
	// Parse optional <> (indicates a static symbol) or
	// <ABIxxx> (selecting text symbol with specific ABI).
	noErrMsg := false
	isStatic, abi := p.symRefAttrs(name, noErrMsg)
	if isStatic {
		return "", obj.ABI0, false // This function rejects static symbols.
	}
	tok = p.next()
	if tok.ScanToken == '+' {
		if p.next().ScanToken != scanner.Int {
			return "", obj.ABI0, false
		}
		tok = p.next()
	}
	if tok.ScanToken != '(' {
		return "", obj.ABI0, false
	}
	if reg := p.next(); reg.ScanToken != scanner.Ident || reg.String() != "SB" {
		return "", obj.ABI0, false
	}
	if p.next().ScanToken != ')' || p.peek() != scanner.EOF {
		return "", obj.ABI0, false
	}
	return name, abi, true
}

// registerIndirect parses the general form of a register indirection.
// It can be (R1), (R2*scale), (R1)(R2*scale), (R1)(R2.SXTX<<3) or (R1)(R2<<3)
// where R1 may be a simple register or register pair R:R or (R, R) or (R+R).
// Or it might be a pseudo-indirection like (FP).
// We are sitting on the opening parenthesis.
func (p *Parser) registerIndirect(a *obj.Addr, prefix rune) {
	p.get('(')
	tok := p.next()
	name := tok.String()
	r1, r2, scale, ok := p.register(name, 0)
	if !ok {
		p.errorf("indirect through non-register %s", tok)
	}
	// SVE extended addressing support
	var ext string
	var mod int64
	var amount int64
	isSVEMemExt := false

	if p.arch.Family == sys.ARM64 && (p.peek() == '.' || p.peek() == lex.LSH) {
		// The index is a vector register, or contains a shift, then this is SVE extended addressing.
		isSVEMemExt = true
		if p.peek() == '.' {
			p.get('.')
			tok2 := p.next()
			str := tok2.String()
			if str == "UXTW" || str == "SXTW" {
				switch str {
				case "UXTW":
					mod = 1
				case "SXTW":
					mod = 2
				}
			} else {
				ext = str
				if p.peek() == '.' {
					p.get('.')
					tok3 := p.next()
					modStr := tok3.String()
					switch modStr {
					case "UXTW":
						mod = 1
					case "SXTW":
						mod = 2
					default:
						p.errorf("unknown modifier %s", modStr)
					}
				}
			}
		}
		if p.peek() == lex.LSH {
			p.get(lex.LSH)
			tok4 := p.get(scanner.Int)
			amount, _ = strconv.ParseInt(tok4.String(), 10, 16)
		}
	}
	p.get(')')
	a.Type = obj.TYPE_MEM

	if isSVEMemExt {
		encodedR1 := r1
		if ext != "" {
			if r1 >= arm64.REG_Z0 && r1 <= arm64.REG_Z31 {
				var arng int
				switch ext {
				case "B":
					arng = arm64.ARNG_B
				case "H":
					arng = arm64.ARNG_H
				case "S":
					arng = arm64.ARNG_S
				case "D":
					arng = arm64.ARNG_D
				case "Q":
					arng = arm64.ARNG_Q
				default:
					p.errorf("unknown arrangement %s", ext)
				}
				if arng != 0 {
					encodedR1 = arm64.REG_ZARNG + (r1 & 31) + int16((arng&15)<<5)
				}
			} else {
				p.errorf("arrangement not allowed for this register")
			}
		}

		if p.peek() != '(' {
			a.Reg = encodedR1
			return
		}
		p.get('(')
		tok5 := p.next()
		r2, ok := p.registerReference(tok5.String())
		if !ok {
			p.errorf("expected register")
		}
		var ext2 string
		if p.peek() == '.' {
			p.get('.')
			tok6 := p.next()
			ext2 = tok6.String()
		}
		p.get(')')

		encodedR2 := r2
		if ext2 != "" {
			if r2 >= arm64.REG_Z0 && r2 <= arm64.REG_Z31 {
				var arng int
				switch ext2 {
				case "B":
					arng = arm64.ARNG_B
				case "H":
					arng = arm64.ARNG_H
				case "S":
					arng = arm64.ARNG_S
				case "D":
					arng = arm64.ARNG_D
				case "Q":
					arng = arm64.ARNG_Q
				default:
					p.errorf("unknown arrangement %s", ext2)
				}
				if arng != 0 {
					encodedR2 = arm64.REG_ZARNG + (r2 & 31) + int16((arng&15)<<5)
				}
			} else {
				p.errorf("arrangement not allowed for this register")
			}
		}

		a.Index = encodedR2 // Base
		a.Reg = encodedR1   // Index
		a.Offset = 0        // Ensure offset is 0

		var scaleValue int16 = -32768 // Bit 15 set
		scaleValue |= int16(amount << 12)
		scaleValue |= int16(mod << 9)
		a.Scale = scaleValue
		return
	}
	if r1 < 0 {
		// Pseudo-register reference.
		if r2 != 0 {
			p.errorf("cannot use pseudo-register in pair")
			return
		}
		// For SB, SP, and FP, there must be a name here. 0(FP) is not legal.
		if name != "PC" && a.Name == obj.NAME_NONE {
			p.errorf("cannot reference %s without a symbol", name)
		}
		p.setPseudoRegister(a, name, false, prefix)
		return
	}
	a.Reg = r1
	if r2 != 0 {
		// TODO: Consistency in the encoding would be nice here.
		if p.arch.InFamily(sys.ARM, sys.ARM64) {
			// Special form
			// ARM: destination register pair (R1, R2).
			// ARM64: register pair (R1, R2) for LDP/STP.
			if prefix != 0 || scale != 0 {
				p.errorf("illegal address mode for register pair")
				return
			}
			a.Type = obj.TYPE_REGREG
			a.Offset = int64(r2)
			// Nothing may follow
			return
		}
		if p.arch.Family == sys.PPC64 {
			// Special form for PPC64: (R1+R2); alias for (R1)(R2).
			if prefix != 0 || scale != 0 {
				p.errorf("illegal address mode for register+register")
				return
			}
			a.Type = obj.TYPE_MEM
			a.Scale = 0
			a.Index = r2
			// Nothing may follow.
			return
		}
	}
	if r2 != 0 {
		p.errorf("indirect through register pair")
	}
	if prefix == '$' {
		a.Type = obj.TYPE_ADDR
	}
	if r1 == arch.RPC && prefix != 0 {
		p.errorf("illegal addressing mode for PC")
	}
	if scale == 0 && p.peek() == '(' {
		// General form (R)(R*scale).
		p.next()
		tok := p.next()
		if p.atRegisterExtension() {
			p.registerExtension(a, tok.String(), prefix)
		} else if p.atRegisterShift() {
			// (R1)(R2<<3)
			p.registerExtension(a, tok.String(), prefix)
		} else {
			r1, r2, scale, ok = p.register(tok.String(), 0)
			if !ok {
				p.errorf("indirect through non-register %s", tok)
			}
			if r2 != 0 {
				p.errorf("unimplemented two-register form")
			}
			a.Index = r1
			if scale != 0 && scale != 1 && (p.arch.Family == sys.ARM64 ||
				p.arch.Family == sys.PPC64) {
				// Support (R1)(R2) (no scaling) and (R1)(R2*1).
				p.errorf("%s doesn't support scaled register format", p.arch.Name)
			} else {
				a.Scale = int16(scale)
			}
		}
		p.get(')')
	} else if scale != 0 {
		if p.arch.Family == sys.ARM64 {
			p.errorf("arm64 doesn't support scaled register format")
		}
		// First (R) was missing, all we have is (R*scale).
		a.Reg = 0
		a.Index = r1
		a.Scale = int16(scale)
	}
}

// registerList parses an ARM or ARM64 register list expression, a list of
// registers in []. There may be comma-separated ranges or individual
// registers, as in [R1,R3-R5] or [V1.S4, V2.S4, V3.S4, V4.S4].
// For ARM, only R0 through R15 may appear.
// For ARM64, V0 through V31 with arrangement may appear.
//
// For 386/AMD64 register list specifies 4VNNIW-style multi-source operand.
// For range of 4 elements, Intel manual uses "+3" notation, for example:
//
//	VP4DPWSSDS zmm1{k1}{z}, zmm2+3, m128
//
// Given asm line:
//
//	VP4DPWSSDS Z5, [Z10-Z13], (AX)
//
// zmm2 is Z10, and Z13 is the only valid value for it (Z10+3).
// Only simple ranges are accepted, like [Z0-Z3].
//
// The opening bracket has been consumed.
func (p *Parser) registerList(a *obj.Addr) {
	if p.arch.InFamily(sys.I386, sys.AMD64) {
		p.registerListX86(a)
	} else {
		p.registerListARM(a)
	}
}

func (p *Parser) registerListARM(a *obj.Addr) {
	// One range per loop.
	var maxReg int
	var bits uint16
	var arrangement int64
	switch p.arch.Family {
	case sys.ARM:
		maxReg = 16
	case sys.ARM64:
		maxReg = 32
	default:
		p.errorf("unexpected register list")
	}
	firstReg := -1
	nextReg := -1
	regCnt := 0
ListLoop:
	for {
		tok := p.next()
		switch tok.ScanToken {
		case ']':
			break ListLoop
		case scanner.EOF:
			p.errorf("missing ']' in register list")
			return
		}
		switch p.arch.Family {
		case sys.ARM64:
			// Vn.T, Zn.T, Pn.T
			name := tok.String()
			r, ok := p.registerReference(name)
			if !ok {
				p.errorf("invalid register: %s", name)
			}
			var registerBase int16
			registerCntMask := 31
			switch name[0] {
			case 'V':
				registerBase = p.arch.Register["V0"]
			case 'Z':
				registerBase = p.arch.Register["Z0"]
			case 'P':
				registerBase = p.arch.Register["P0"]
				registerCntMask = 15
			default:
				p.errorf("invalid register in register list: %s", name)
			}
			reg := r - registerBase
			p.get('.')
			tok := p.next()
			ext := tok.String()
			curArrangement, err := arch.ARM64RegisterArrangement(reg, name, ext)
			if err != nil {
				p.errorf("%v", err)
			}
			if firstReg == -1 {
				// only record the first register and arrangement
				firstReg = int(reg)
				nextReg = firstReg
				arrangement = curArrangement
			} else if curArrangement != arrangement {
				p.errorf("inconsistent arrangement in ARM64 register list")
			} else if nextReg != int(reg) {
				p.errorf("incontiguous register in ARM64 register list: %s", name)
			}
			if p.peek() == '-' && name[0] == 'Z' {
				// register range in SVE
				p.next()
				nameHi := p.next().String()
				if nameHi[0] != 'Z' {
					p.errorf("invalid register in register range: %s", nameHi)
				}
				rHi, ok2 := p.registerReference(nameHi)
				if !ok2 {
					p.errorf("invalid register: %s", nameHi)
				}
				regHi := rHi - registerBase
				p.get('.')
				extHi := p.next().String()
				hiArrangement, err := arch.ARM64RegisterArrangement(regHi, nameHi, extHi)
				if err != nil {
					p.errorf("%v", err)
				}
				if hiArrangement != arrangement {
					p.errorf("inconsistent arrangement in ARM64 register list range")
				}
				// Scale stores the range size - 1.
				a.Scale = regHi - reg
			} else {
				regCnt++
				nextReg = (nextReg + 1) & registerCntMask
			}
		case sys.ARM:
			// Parse the upper and lower bounds.
			lo := p.registerNumber(tok.String())
			hi := lo
			if p.peek() == '-' {
				p.next()
				hi = p.registerNumber(p.next().String())
			}
			if hi < lo {
				lo, hi = hi, lo
			}
			// Check there are no duplicates in the register list.
			for i := 0; lo <= hi && i < maxReg; i++ {
				if bits&(1<<lo) != 0 {
					p.errorf("register R%d already in list", lo)
				}
				bits |= 1 << lo
				lo++
			}
		default:
			p.errorf("unexpected register list")
		}
		if p.peek() != ']' {
			p.get(',')
		}
	}
	a.Type = obj.TYPE_REGLIST
	switch p.arch.Family {
	case sys.ARM:
		a.Offset = int64(bits)
	case sys.ARM64:
		offset, err := arm64.RegisterListOffset(firstReg, regCnt, arrangement, a.Scale)
		if err != nil {
			p.errorf("%v", err)
		}
		a.Offset = offset
	default:
		p.errorf("register list not supported on this architecture")
	}
}

func (p *Parser) registerListX86(a *obj.Addr) {
	// Accept only [RegA-RegB] syntax.
	// Don't use p.get() to provide better error messages.

	loName := p.next().String()
	lo, ok := p.arch.Register[loName]
	if !ok {
		if loName == "EOF" {
			p.errorf("register list: expected ']', found EOF")
		} else {
			p.errorf("register list: bad low register in `[%s`", loName)
		}
		return
	}
	if tok := p.next().ScanToken; tok != '-' {
		p.errorf("register list: expected '-' after `[%s`, found %s", loName, tok)
		return
	}
	hiName := p.next().String()
	hi, ok := p.arch.Register[hiName]
	if !ok {
		p.errorf("register list: bad high register in `[%s-%s`", loName, hiName)
		return
	}
	if tok := p.next().ScanToken; tok != ']' {
		p.errorf("register list: expected ']' after `[%s-%s`, found %s", loName, hiName, tok)
	}

	a.Type = obj.TYPE_REGLIST
	a.Reg = lo
	a.Offset = x86.EncodeRegisterRange(lo, hi)
}

// registerNumber is ARM-specific. It returns the number of the specified register.
func (p *Parser) registerNumber(name string) uint16 {
	if p.arch.Family == sys.ARM && name == "g" {
		return 10
	}
	if name[0] != 'R' {
		p.errorf("expected g or R0 through R15; found %s", name)
		return 0
	}
	r, ok := p.registerReference(name)
	if !ok {
		return 0
	}
	reg := r - p.arch.Register["R0"]
	if reg < 0 {
		// Could happen for an architecture having other registers prefixed by R
		p.errorf("expected g or R0 through R15; found %s", name)
		return 0
	}
	return uint16(reg)
}

// Note: There are two changes in the expression handling here
// compared to the old yacc/C implementations. Neither has
// much practical consequence because the expressions we
// see in assembly code are simple, but for the record:
//
// 1) Evaluation uses uint64; the old one used int64.
// 2) Precedence uses Go rules not C rules.

// expr = term | term ('+' | '-' | '|' | '^') term.
func (p *Parser) expr() uint64 {
	value := p.term()
	for {
		switch p.peek() {
		case '+':
			p.next()
			value += p.term()
		case '-':
			p.next()
			value -= p.term()
		case '|':
			p.next()
			value |= p.term()
		case '^':
			p.next()
			value ^= p.term()
		default:
			return value
		}
	}
}

// floatExpr = fconst | '-' floatExpr | '+' floatExpr | '(' floatExpr ')'
func (p *Parser) floatExpr() float64 {
	tok := p.next()
	switch tok.ScanToken {
	case '(':
		v := p.floatExpr()
		if p.next().ScanToken != ')' {
			p.errorf("missing closing paren")
		}
		return v
	case '+':
		return +p.floatExpr()
	case '-':
		return -p.floatExpr()
	case scanner.Float:
		return p.atof(tok.String())
	}
	p.errorf("unexpected %s evaluating float expression", tok)
	return 0
}

// term = factor | factor ('*' | '/' | '%' | '>>' | '<<' | '&') factor
func (p *Parser) term() uint64 {
	value := p.factor()
	for {
		switch p.peek() {
		case '*':
			p.next()
			value *= p.factor()
		case '/':
			p.next()
			if int64(value) < 0 {
				p.errorf("divide of value with high bit set")
			}
			divisor := p.factor()
			if divisor == 0 {
				p.errorf("division by zero")
			} else {
				value /= divisor
			}
		case '%':
			p.next()
			divisor := p.factor()
			if int64(value) < 0 {
				p.errorf("modulo of value with high bit set")
			}
			if divisor == 0 {
				p.errorf("modulo by zero")
			} else {
				value %= divisor
			}
		case lex.LSH:
			p.next()
			shift := p.factor()
			if int64(shift) < 0 {
				p.errorf("negative left shift count")
			}
			return value << shift
		case lex.RSH:
			p.next()
			shift := p.term()
			if int64(shift) < 0 {
				p.errorf("negative right shift count")
			}
			if int64(value) < 0 {
				p.errorf("right shift of value with high bit set")
			}
			value >>= shift
		case '&':
			p.next()
			value &= p.factor()
		default:
			return value
		}
	}
}

// factor = const | '+' factor | '-' factor | '~' factor | '(' expr ')'
func (p *Parser) factor() uint64 {
	tok := p.next()
	switch tok.ScanToken {
	case scanner.Int:
		return p.atoi(tok.String())
	case scanner.Char:
		str, err := strconv.Unquote(tok.String())
		if err != nil {
			p.errorf("%s", err)
		}
		r, w := utf8.DecodeRuneInString(str)
		if w == 1 && r == utf8.RuneError {
			p.errorf("illegal UTF-8 encoding for character constant")
		}
		return uint64(r)
	case '+':
		return +p.factor()
	case '-':
		return -p.factor()
	case '~':
		return ^p.factor()
	case '(':
		v := p.expr()
		if p.next().ScanToken != ')' {
			p.errorf("missing closing paren")
		}
		return v
	}
	p.errorf("unexpected %s evaluating expression", tok)
	return 0
}

// positiveAtoi returns an int64 that must be >= 0.
func (p *Parser) positiveAtoi(str string) int64 {
	value, err := strconv.ParseInt(str, 0, 64)
	if err != nil {
		p.errorf("%s", err)
	}
	if value < 0 {
		p.errorf("%s overflows int64", str)
	}
	return value
}

func (p *Parser) atoi(str string) uint64 {
	value, err := strconv.ParseUint(str, 0, 64)
	if err != nil {
		p.errorf("%s", err)
	}
	return value
}

func (p *Parser) atof(str string) float64 {
	value, err := strconv.ParseFloat(str, 64)
	if err != nil {
		p.errorf("%s", err)
	}
	return value
}

// EOF represents the end of input.
var EOF = lex.Make(scanner.EOF, "EOF")

func (p *Parser) next() lex.Token {
	if !p.more() {
		return EOF
	}
	tok := p.input[p.inputPos]
	p.inputPos++
	return tok
}

func (p *Parser) back() {
	if p.inputPos == 0 {
		p.errorf("internal error: backing up before BOL")
	} else {
		p.inputPos--
	}
}

func (p *Parser) peek() lex.ScanToken {
	if p.more() {
		return p.input[p.inputPos].ScanToken
	}
	return scanner.EOF
}

func (p *Parser) more() bool {
	return p.inputPos < len(p.input)
}

// get verifies that the next item has the expected type and returns it.
func (p *Parser) get(expected lex.ScanToken) lex.Token {
	p.expect(expected, expected.String())
	return p.next()
}

// expectOperandEnd verifies that the parsing state is properly at the end of an operand.
func (p *Parser) expectOperandEnd() {
	p.expect(scanner.EOF, "end of operand")
}

// expect verifies that the next item has the expected type. It does not consume it.
func (p *Parser) expect(expectedToken lex.ScanToken, expectedMessage string) {
	if p.peek() != expectedToken {
		p.errorf("expected %s, found %s", expectedMessage, p.next())
	}
}

// have reports whether the remaining tokens (including the current one) contain the specified token.
func (p *Parser) have(token lex.ScanToken) bool {
	for i := p.inputPos; i < len(p.input); i++ {
		if p.input[i].ScanToken == token {
			return true
		}
	}
	return false
}

// at reports whether the next tokens are as requested.
func (p *Parser) at(next ...lex.ScanToken) bool {
	if len(p.input)-p.inputPos < len(next) {
		return false
	}
	for i, r := range next {
		if p.input[p.inputPos+i].ScanToken != r {
			return false
		}
	}
	return true
}

```

// === FILE: references!/go/src/cmd/asm/internal/flags/flags.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package flags implements top-level flags and the usage message for the assembler.
package flags

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	Debug      = flag.Bool("debug", false, "dump instructions as they are parsed")
	OutputFile = flag.String("o", "", "output file; default foo.o for /a/b/c/foo.s as first argument")
	TrimPath   = flag.String("trimpath", "", "remove prefix from recorded source file paths")
	Shared     = flag.Bool("shared", false, "generate code that can be linked into a shared library")
	Dynlink    = flag.Bool("dynlink", false, "support references to Go symbols defined in other shared libraries")
	Linkshared = flag.Bool("linkshared", false, "generate code that will be linked against Go shared libraries")
	AllErrors  = flag.Bool("e", false, "no limit on number of errors reported")
	SymABIs    = flag.Bool("gensymabis", false, "write symbol ABI information to output file, don't assemble")
	Importpath = flag.String("p", obj.UnlinkablePkg, "set expected package import to path")
	Spectre    = flag.String("spectre", "", "enable spectre mitigations in `list` (all, ret)")
	Std        = flag.Bool("std", false, "building standard library")
)

var DebugFlags struct {
	CompressInstructions int    `help:"use compressed instructions when possible (if supported by architecture)"`
	MayMoreStack         string `help:"call named function before all stack growth checks"`
	PCTab                string `help:"print named pc-value table\nOne of: pctospadj, pctofile, pctoline, pctoinline, pctopcdata"`
}

var (
	D        MultiFlag
	I        MultiFlag
	PrintOut int
	DebugV   bool
)

func init() {
	flag.Var(&D, "D", "predefined symbol with optional simple value -D=identifier=value; can be set multiple times")
	flag.Var(&I, "I", "include directory; can be set multiple times")
	flag.BoolVar(&DebugV, "v", false, "print debug output")
	flag.Var(objabi.NewDebugFlag(&DebugFlags, nil), "d", "enable debugging settings; try -d help")
	objabi.AddVersionFlag() // -V
	objabi.Flagcount("S", "print assembly and machine code", &PrintOut)

	DebugFlags.CompressInstructions = 1
}

// MultiFlag allows setting a value multiple times to collect a list, as in -I=dir1 -I=dir2.
type MultiFlag []string

func (m *MultiFlag) String() string {
	if len(*m) == 0 {
		return ""
	}
	return fmt.Sprint(*m)
}

func (m *MultiFlag) Set(val string) error {
	(*m) = append(*m, val)
	return nil
}

func Usage() {
	fmt.Fprintf(os.Stderr, "usage: asm [options] file.s ...\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func Parse() {
	objabi.Flagparse(Usage)
	if flag.NArg() == 0 {
		flag.Usage()
	}

	// Flag refinement.
	if *OutputFile == "" {
		if flag.NArg() != 1 {
			flag.Usage()
		}
		input := filepath.Base(flag.Arg(0))
		input = strings.TrimSuffix(input, ".s")
		*OutputFile = fmt.Sprintf("%s.o", input)
	}
}

```

// === FILE: references!/go/src/cmd/asm/internal/lex/input.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lex

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/scanner"

	"cmd/asm/internal/flags"
	"cmd/internal/objabi"
	"cmd/internal/src"
)

// Input is the main input: a stack of readers and some macro definitions.
// It also handles #include processing (by pushing onto the input stack)
// and parses and instantiates macro definitions.
type Input struct {
	Stack
	includes        []string
	beginningOfLine bool
	ifdefStack      []bool
	macros          map[string]*Macro
	text            string // Text of last token returned by Next.
	peek            bool
	peekToken       ScanToken
	peekText        string
}

// NewInput returns an Input from the given path.
func NewInput(name string) *Input {
	return &Input{
		// include directories: look in source dir, then -I directories.
		includes:        append([]string{filepath.Dir(name)}, flags.I...),
		beginningOfLine: true,
		macros:          predefine(flags.D),
	}
}

// predefine installs the macros set by the -D flag on the command line.
func predefine(defines flags.MultiFlag) map[string]*Macro {
	macros := make(map[string]*Macro)
	for _, name := range defines {
		value := "1"
		i := strings.IndexRune(name, '=')
		if i > 0 {
			name, value = name[:i], name[i+1:]
		}
		tokens := Tokenize(name)
		if len(tokens) != 1 || tokens[0].ScanToken != scanner.Ident {
			fmt.Fprintf(os.Stderr, "asm: parsing -D: %q is not a valid identifier name\n", tokens[0])
			flags.Usage()
		}
		macros[name] = &Macro{
			name:   name,
			args:   nil,
			tokens: Tokenize(value),
		}
	}
	return macros
}

var panicOnError bool // For testing.

func (in *Input) Error(args ...any) {
	if panicOnError {
		panic(fmt.Errorf("%s:%d: %s", in.File(), in.Line(), fmt.Sprintln(args...)))
	}
	fmt.Fprintf(os.Stderr, "%s:%d: %s", in.File(), in.Line(), fmt.Sprintln(args...))
	os.Exit(1)
}

// expectText is like Error but adds "got XXX" where XXX is a quoted representation of the most recent token.
func (in *Input) expectText(args ...any) {
	in.Error(append(args, "; got", strconv.Quote(in.Stack.Text()))...)
}

// enabled reports whether the input is enabled by an ifdef, or is at the top level.
func (in *Input) enabled() bool {
	return len(in.ifdefStack) == 0 || in.ifdefStack[len(in.ifdefStack)-1]
}

func (in *Input) expectNewline(directive string) {
	tok := in.Stack.Next()
	if tok != '\n' {
		in.expectText("expected newline after", directive)
	}
}

func (in *Input) Next() ScanToken {
	if in.peek {
		in.peek = false
		tok := in.peekToken
		in.text = in.peekText
		return tok
	}
	// If we cannot generate a token after 100 macro invocations, we're in trouble.
	// The usual case is caught by Push, below, but be safe.
	for nesting := 0; nesting < 100; {
		tok := in.Stack.Next()
		switch tok {
		case '#':
			if !in.beginningOfLine {
				in.Error("'#' must be first item on line")
			}
			in.beginningOfLine = in.hash()
			in.text = "#"
			return '#'

		case scanner.Ident:
			// Is it a macro name?
			name := in.Stack.Text()
			macro := in.macros[name]
			if macro != nil {
				nesting++
				in.invokeMacro(macro)
				continue
			}
			fallthrough
		default:
			if tok == scanner.EOF && len(in.ifdefStack) > 0 {
				// We're skipping text but have run out of input with no #endif.
				in.Error("unclosed #ifdef or #ifndef")
			}
			in.beginningOfLine = tok == '\n'
			if in.enabled() {
				in.text = in.Stack.Text()
				return tok
			}
		}
	}
	in.Error("recursive macro invocation")
	return 0
}

func (in *Input) Text() string {
	return in.text
}

// hash processes a # preprocessor directive. It reports whether it completes.
func (in *Input) hash() bool {
	// We have a '#'; it must be followed by a known word (define, include, etc.).
	tok := in.Stack.Next()
	if tok != scanner.Ident {
		in.expectText("expected identifier after '#'")
	}
	if !in.enabled() {
		// Can only start including again if we are at #else or #endif but also
		// need to keep track of nested #if[n]defs.
		// We let #line through because it might affect errors.
		switch in.Stack.Text() {
		case "else", "endif", "ifdef", "ifndef", "line":
			// Press on.
		default:
			return false
		}
	}
	switch in.Stack.Text() {
	case "define":
		in.define()
	case "else":
		in.else_()
	case "endif":
		in.endif()
	case "ifdef":
		in.ifdef(true)
	case "ifndef":
		in.ifdef(false)
	case "include":
		in.include()
	case "line":
		in.line()
	case "undef":
		in.undef()
	default:
		in.Error("unexpected token after '#':", in.Stack.Text())
	}
	return true
}

// macroName returns the name for the macro being referenced.
func (in *Input) macroName() string {
	// We use the Stack's input method; no macro processing at this stage.
	tok := in.Stack.Next()
	if tok != scanner.Ident {
		in.expectText("expected identifier after # directive")
	}
	// Name is alphanumeric by definition.
	return in.Stack.Text()
}

// #define processing.
func (in *Input) define() {
	name := in.macroName()
	args, tokens := in.macroDefinition(name)
	in.defineMacro(name, args, tokens)
}

// defineMacro stores the macro definition in the Input.
func (in *Input) defineMacro(name string, args []string, tokens []Token) {
	if in.macros[name] != nil {
		in.Error("redefinition of macro:", name)
	}
	in.macros[name] = &Macro{
		name:   name,
		args:   args,
		tokens: tokens,
	}
}

// macroDefinition returns the list of formals and the tokens of the definition.
// The argument list is nil for no parens on the definition; otherwise a list of
// formal argument names.
func (in *Input) macroDefinition(name string) ([]string, []Token) {
	prevCol := in.Stack.Col()
	tok := in.Stack.Next()
	if tok == '\n' || tok == scanner.EOF {
		return nil, nil // No definition for macro
	}
	var args []string
	// The C preprocessor treats
	//	#define A(x)
	// and
	//	#define A (x)
	// distinctly: the first is a macro with arguments, the second without.
	// Distinguish these cases using the column number, since we don't
	// see the space itself. Note that text/scanner reports the position at the
	// end of the token. It's where you are now, and you just read this token.
	if tok == '(' && in.Stack.Col() == prevCol+1 {
		// Macro has arguments. Scan list of formals.
		acceptArg := true
		args = []string{} // Zero length but not nil.
	Loop:
		for {
			tok = in.Stack.Next()
			switch tok {
			case ')':
				tok = in.Stack.Next() // First token of macro definition.
				break Loop
			case ',':
				if acceptArg {
					in.Error("bad syntax in definition for macro:", name)
				}
				acceptArg = true
			case scanner.Ident:
				if !acceptArg {
					in.Error("bad syntax in definition for macro:", name)
				}
				arg := in.Stack.Text()
				if slices.Contains(args, arg) {
					in.Error("duplicate argument", arg, "in definition for macro:", name)
				}
				args = append(args, arg)
				acceptArg = false
			default:
				in.Error("bad definition for macro:", name)
			}
		}
	}
	var tokens []Token
	// Scan to newline. Backslashes escape newlines.
	for tok != '\n' {
		if tok == scanner.EOF {
			in.Error("missing newline in definition for macro:", name)
		}
		if tok == '\\' {
			tok = in.Stack.Next()
			if tok != '\n' && tok != '\\' {
				in.Error(`can only escape \ or \n in definition for macro:`, name)
			}
		}
		tokens = append(tokens, Make(tok, in.Stack.Text()))
		tok = in.Stack.Next()
	}
	return args, tokens
}

// invokeMacro pushes onto the input Stack a Slice that holds the macro definition with the actual
// parameters substituted for the formals.
// Invoking a macro does not touch the PC/line history.
func (in *Input) invokeMacro(macro *Macro) {
	// If the macro has no arguments, just substitute the text.
	if macro.args == nil {
		in.Push(NewSlice(in.Base(), in.Line(), macro.tokens))
		return
	}
	tok := in.Stack.Next()
	if tok != '(' {
		// If the macro has arguments but is invoked without them, all we push is the macro name.
		// First, put back the token.
		in.peekToken = tok
		in.peekText = in.text
		in.peek = true
		in.Push(NewSlice(in.Base(), in.Line(), []Token{Make(macroName, macro.name)}))
		return
	}
	actuals := in.argsFor(macro)
	var tokens []Token
	for _, tok := range macro.tokens {
		if tok.ScanToken != scanner.Ident {
			tokens = append(tokens, tok)
			continue
		}
		substitution := actuals[tok.text]
		if substitution == nil {
			tokens = append(tokens, tok)
			continue
		}
		tokens = append(tokens, substitution...)
	}
	in.Push(NewSlice(in.Base(), in.Line(), tokens))
}

// argsFor returns a map from formal name to actual value for this argumented macro invocation.
// The opening parenthesis has been absorbed.
func (in *Input) argsFor(macro *Macro) map[string][]Token {
	var args [][]Token
	// One macro argument per iteration. Collect them all and check counts afterwards.
	for argNum := 0; ; argNum++ {
		tokens, tok := in.collectArgument(macro)
		args = append(args, tokens)
		if tok == ')' {
			break
		}
	}
	// Zero-argument macros are tricky.
	if len(macro.args) == 0 && len(args) == 1 && args[0] == nil {
		args = nil
	} else if len(args) != len(macro.args) {
		in.Error("wrong arg count for macro", macro.name)
	}
	argMap := make(map[string][]Token)
	for i, arg := range args {
		argMap[macro.args[i]] = arg
	}
	return argMap
}

// collectArgument returns the actual tokens for a single argument of a macro.
// It also returns the token that terminated the argument, which will always
// be either ',' or ')'. The starting '(' has been scanned.
func (in *Input) collectArgument(macro *Macro) ([]Token, ScanToken) {
	nesting := 0
	var tokens []Token
	for {
		tok := in.Stack.Next()
		if tok == scanner.EOF || tok == '\n' {
			in.Error("unterminated arg list invoking macro:", macro.name)
		}
		if nesting == 0 && (tok == ')' || tok == ',') {
			return tokens, tok
		}
		if tok == '(' {
			nesting++
		}
		if tok == ')' {
			nesting--
		}
		tokens = append(tokens, Make(tok, in.Stack.Text()))
	}
}

// #ifdef and #ifndef processing.
func (in *Input) ifdef(truth bool) {
	name := in.macroName()
	in.expectNewline("#if[n]def")
	if !in.enabled() {
		truth = false
	} else if _, defined := in.macros[name]; !defined {
		truth = !truth
	}
	in.ifdefStack = append(in.ifdefStack, truth)
}

// #else processing
func (in *Input) else_() {
	in.expectNewline("#else")
	if len(in.ifdefStack) == 0 {
		in.Error("unmatched #else")
	}
	if len(in.ifdefStack) == 1 || in.ifdefStack[len(in.ifdefStack)-2] {
		in.ifdefStack[len(in.ifdefStack)-1] = !in.ifdefStack[len(in.ifdefStack)-1]
	}
}

// #endif processing.
func (in *Input) endif() {
	in.expectNewline("#endif")
	if len(in.ifdefStack) == 0 {
		in.Error("unmatched #endif")
	}
	in.ifdefStack = in.ifdefStack[:len(in.ifdefStack)-1]
}

// #include processing.
func (in *Input) include() {
	// Find and parse string.
	tok := in.Stack.Next()
	if tok != scanner.String {
		in.expectText("expected string after #include")
	}
	name, err := strconv.Unquote(in.Stack.Text())
	if err != nil {
		in.Error("unquoting include file name: ", err)
	}
	in.expectNewline("#include")
	// Push tokenizer for file onto stack.
	fd, err := os.Open(name)
	if err != nil {
		for _, dir := range in.includes {
			fd, err = os.Open(filepath.Join(dir, name))
			if err == nil {
				break
			}
		}
		if err != nil {
			in.Error("#include:", err)
		}
	}
	in.Push(NewTokenizer(name, fd, fd))
}

// #line processing.
func (in *Input) line() {
	// Only need to handle Plan 9 format: #line 337 "filename"
	tok := in.Stack.Next()
	if tok != scanner.Int {
		in.expectText("expected line number after #line")
	}
	line, err := strconv.Atoi(in.Stack.Text())
	if err != nil {
		in.Error("error parsing #line (cannot happen):", err)
	}
	tok = in.Stack.Next()
	if tok != scanner.String {
		in.expectText("expected file name in #line")
	}
	file, err := strconv.Unquote(in.Stack.Text())
	if err != nil {
		in.Error("unquoting #line file name: ", err)
	}
	tok = in.Stack.Next()
	if tok != '\n' {
		in.Error("unexpected token at end of #line: ", tok)
	}
	pos := src.MakePos(in.Base(), uint(in.Line())+1, 1) // +1 because #line nnn means line nnn starts on next line
	in.Stack.SetBase(src.NewLinePragmaBase(pos, file, objabi.AbsFile(objabi.WorkingDir(), file, *flags.TrimPath), uint(line), 1))
}

// #undef processing
func (in *Input) undef() {
	name := in.macroName()
	if in.macros[name] == nil {
		in.Error("#undef for undefined macro:", name)
	}
	// Newline must be next.
	tok := in.Stack.Next()
	if tok != '\n' {
		in.Error("syntax error in #undef for macro:", name)
	}
	delete(in.macros, name)
}

func (in *Input) Push(r TokenReader) {
	if len(in.tr) > 100 {
		in.Error("input recursion")
	}
	in.Stack.Push(r)
}

func (in *Input) Close() {
}

```

// === FILE: references!/go/src/cmd/asm/internal/lex/lex.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lex implements lexical analysis for the assembler.
package lex

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/scanner"

	"cmd/internal/src"
)

// A ScanToken represents an input item. It is a simple wrapping of rune, as
// returned by text/scanner.Scanner, plus a couple of extra values.
type ScanToken rune

const (
	// Asm defines some two-character lexemes. We make up
	// a rune/ScanToken value for them - ugly but simple.
	LSH          ScanToken = -1000 - iota // << Left shift.
	RSH                                   // >> Logical right shift.
	ARR                                   // -> Used on ARM for shift type 3, arithmetic right shift.
	ROT                                   // @> Used on ARM for shift type 4, rotate right.
	Include                               // included file started here
	BuildComment                          // //go:build or +build comment
	macroName                             // name of macro that should not be expanded
)

// IsRegisterShift reports whether the token is one of the ARM register shift operators.
func IsRegisterShift(r ScanToken) bool {
	return ROT <= r && r <= LSH // Order looks backwards because these are negative.
}

func (t ScanToken) String() string {
	switch t {
	case scanner.EOF:
		return "EOF"
	case scanner.Ident:
		return "identifier"
	case scanner.Int:
		return "integer constant"
	case scanner.Float:
		return "float constant"
	case scanner.Char:
		return "rune constant"
	case scanner.String:
		return "string constant"
	case scanner.RawString:
		return "raw string constant"
	case scanner.Comment:
		return "comment"
	default:
		return fmt.Sprintf("%q", rune(t))
	}
}

// NewLexer returns a lexer for the named file and the given link context.
func NewLexer(name string) TokenReader {
	input := NewInput(name)
	fd, err := os.Open(name)
	if err != nil {
		log.Fatalf("%s\n", err)
	}
	input.Push(NewTokenizer(name, fd, fd))
	return input
}

// The other files in this directory each contain an implementation of TokenReader.

// A TokenReader is like a reader, but returns lex tokens of type Token. It also can tell you what
// the text of the most recently returned token is, and where it was found.
// The underlying scanner elides all spaces except newline, so the input looks like a stream of
// Tokens; original spacing is lost but we don't need it.
type TokenReader interface {
	// Next returns the next token.
	Next() ScanToken
	// The following methods all refer to the most recent token returned by Next.
	// Text returns the original string representation of the token.
	Text() string
	// File reports the source file name of the token.
	File() string
	// Base reports the position base of the token.
	Base() *src.PosBase
	// SetBase sets the position base.
	SetBase(*src.PosBase)
	// Line reports the source line number of the token.
	Line() int
	// Col reports the source column number of the token.
	Col() int
	// Close does any teardown required.
	Close()
}

// A Token is a scan token plus its string value.
// A macro is stored as a sequence of Tokens with spaces stripped.
type Token struct {
	ScanToken
	text string
}

// Make returns a Token with the given rune (ScanToken) and text representation.
func Make(token ScanToken, text string) Token {
	// Substitute the substitutes for . and /.
	text = strings.ReplaceAll(text, "\u00B7", ".")
	text = strings.ReplaceAll(text, "\u2215", "/")
	return Token{ScanToken: token, text: text}
}

func (l Token) String() string {
	return l.text
}

// A Macro represents the definition of a #defined macro.
type Macro struct {
	name   string   // The #define name.
	args   []string // Formal arguments.
	tokens []Token  // Body of macro.
}

// Tokenize turns a string into a list of Tokens; used to parse the -D flag and in tests.
func Tokenize(str string) []Token {
	t := NewTokenizer("command line", strings.NewReader(str), nil)
	var tokens []Token
	for {
		tok := t.Next()
		if tok == scanner.EOF {
			break
		}
		tokens = append(tokens, Make(tok, t.Text()))
	}
	return tokens
}

```

// === FILE: references!/go/src/cmd/asm/internal/lex/slice.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lex

import (
	"text/scanner"

	"cmd/internal/src"
)

// A Slice reads from a slice of Tokens.
type Slice struct {
	tokens []Token
	base   *src.PosBase
	line   int
	pos    int
}

func NewSlice(base *src.PosBase, line int, tokens []Token) *Slice {
	return &Slice{
		tokens: tokens,
		base:   base,
		line:   line,
		pos:    -1, // Next will advance to zero.
	}
}

func (s *Slice) Next() ScanToken {
	s.pos++
	if s.pos >= len(s.tokens) {
		return scanner.EOF
	}
	return s.tokens[s.pos].ScanToken
}

func (s *Slice) Text() string {
	return s.tokens[s.pos].text
}

func (s *Slice) File() string {
	return s.base.Filename()
}

func (s *Slice) Base() *src.PosBase {
	return s.base
}

func (s *Slice) SetBase(base *src.PosBase) {
	// Cannot happen because we only have slices of already-scanned text,
	// but be prepared.
	s.base = base
}

func (s *Slice) Line() int {
	return s.line
}

func (s *Slice) Col() int {
	// TODO: Col is only called when defining a macro and all it cares about is increasing
	// position to discover whether there is a blank before the parenthesis.
	// We only get here if defining a macro inside a macro.
	// This imperfect implementation means we cannot tell the difference between
	//	#define A #define B(x) x
	// and
	//	#define A #define B (x) x
	// The first definition of B has an argument, the second doesn't. Because we let
	// text/scanner strip the blanks for us, this is extremely rare, hard to fix, and not worth it.
	return s.pos
}

func (s *Slice) Close() {
}

```

// === FILE: references!/go/src/cmd/asm/internal/lex/stack.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lex

import (
	"text/scanner"

	"cmd/internal/src"
)

// A Stack is a stack of TokenReaders. As the top TokenReader hits EOF,
// it resumes reading the next one down.
type Stack struct {
	tr []TokenReader
}

// Push adds tr to the top (end) of the input stack. (Popping happens automatically.)
func (s *Stack) Push(tr TokenReader) {
	s.tr = append(s.tr, tr)
}

func (s *Stack) Next() ScanToken {
	tos := s.tr[len(s.tr)-1]
	tok := tos.Next()
	for tok == scanner.EOF && len(s.tr) > 1 {
		tos.Close()
		// Pop the topmost item from the stack and resume with the next one down.
		s.tr = s.tr[:len(s.tr)-1]
		tok = s.Next()
	}
	return tok
}

func (s *Stack) Text() string {
	return s.tr[len(s.tr)-1].Text()
}

func (s *Stack) File() string {
	return s.Base().Filename()
}

func (s *Stack) Base() *src.PosBase {
	return s.tr[len(s.tr)-1].Base()
}

func (s *Stack) SetBase(base *src.PosBase) {
	s.tr[len(s.tr)-1].SetBase(base)
}

func (s *Stack) Line() int {
	return s.tr[len(s.tr)-1].Line()
}

func (s *Stack) Col() int {
	return s.tr[len(s.tr)-1].Col()
}

func (s *Stack) Close() { // Unused.
}

```

// === FILE: references!/go/src/cmd/asm/internal/lex/tokenizer.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lex

import (
	"go/build/constraint"
	"io"
	"os"
	"strings"
	"text/scanner"
	"unicode"

	"cmd/asm/internal/flags"
	"cmd/internal/objabi"
	"cmd/internal/src"
)

// A Tokenizer is a simple wrapping of text/scanner.Scanner, configured
// for our purposes and made a TokenReader. It forms the lowest level,
// turning text from readers into tokens.
type Tokenizer struct {
	tok  ScanToken
	s    *scanner.Scanner
	base *src.PosBase
	line int
	file *os.File // If non-nil, file descriptor to close.
}

func NewTokenizer(name string, r io.Reader, file *os.File) *Tokenizer {
	var s scanner.Scanner
	s.Init(r)
	// Newline is like a semicolon; other space characters are fine.
	s.Whitespace = 1<<'\t' | 1<<'\r' | 1<<' '
	// Don't skip comments: we need to count newlines.
	s.Mode = scanner.ScanChars |
		scanner.ScanFloats |
		scanner.ScanIdents |
		scanner.ScanInts |
		scanner.ScanStrings |
		scanner.ScanComments
	s.Position.Filename = name
	s.IsIdentRune = isIdentRune
	return &Tokenizer{
		s:    &s,
		base: src.NewFileBase(name, objabi.AbsFile(objabi.WorkingDir(), name, *flags.TrimPath)),
		line: 1,
		file: file,
	}
}

// We want center dot (·) and division slash (∕) to work as identifier characters.
func isIdentRune(ch rune, i int) bool {
	if unicode.IsLetter(ch) {
		return true
	}
	switch ch {
	case '_': // Underscore; traditional.
		return true
	case '\u00B7': // Represents the period in runtime.exit. U+00B7 '·' middle dot
		return true
	case '\u2215': // Represents the slash in runtime/debug.setGCPercent. U+2215 '∕' division slash
		return true
	}
	// Digits are OK only after the first character.
	return i > 0 && unicode.IsDigit(ch)
}

func (t *Tokenizer) Text() string {
	switch t.tok {
	case LSH:
		return "<<"
	case RSH:
		return ">>"
	case ARR:
		return "->"
	case ROT:
		return "@>"
	}
	return t.s.TokenText()
}

func (t *Tokenizer) File() string {
	return t.base.Filename()
}

func (t *Tokenizer) Base() *src.PosBase {
	return t.base
}

func (t *Tokenizer) SetBase(base *src.PosBase) {
	t.base = base
}

func (t *Tokenizer) Line() int {
	return t.line
}

func (t *Tokenizer) Col() int {
	return t.s.Pos().Column
}

func (t *Tokenizer) Next() ScanToken {
	s := t.s
	for {
		t.tok = ScanToken(s.Scan())
		if t.tok != scanner.Comment {
			break
		}
		text := s.TokenText()
		t.line += strings.Count(text, "\n")
		if constraint.IsGoBuild(text) {
			t.tok = BuildComment
			break
		}
	}
	switch t.tok {
	case '\n':
		t.line++
	case '-':
		if s.Peek() == '>' {
			s.Next()
			t.tok = ARR
			return ARR
		}
	case '@':
		if s.Peek() == '>' {
			s.Next()
			t.tok = ROT
			return ROT
		}
	case '<':
		if s.Peek() == '<' {
			s.Next()
			t.tok = LSH
			return LSH
		}
	case '>':
		if s.Peek() == '>' {
			s.Next()
			t.tok = RSH
			return RSH
		}
	}
	return t.tok
}

func (t *Tokenizer) Close() {
	if t.file != nil {
		t.file.Close()
	}
}

```

// === FILE: references!/go/src/cmd/asm/main.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"internal/buildcfg"
	"log"
	"os"

	"cmd/asm/internal/arch"
	"cmd/asm/internal/asm"
	"cmd/asm/internal/flags"
	"cmd/asm/internal/lex"

	"cmd/internal/bio"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/telemetry/counter"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("asm: ")
	counter.Open()

	buildcfg.Check()
	GOARCH := buildcfg.GOARCH

	flags.Parse()
	counter.Inc("asm/invocations")
	counter.CountFlags("asm/flag:", *flag.CommandLine)

	architecture := arch.Set(GOARCH, *flags.Shared || *flags.Dynlink)
	if architecture == nil {
		log.Fatalf("unrecognized architecture %s", GOARCH)
	}
	ctxt := obj.Linknew(architecture.LinkArch)
	ctxt.CompressInstructions = flags.DebugFlags.CompressInstructions != 0
	ctxt.Debugasm = flags.PrintOut
	ctxt.Debugvlog = flags.DebugV
	ctxt.Flag_dynlink = *flags.Dynlink
	ctxt.Flag_linkshared = *flags.Linkshared
	ctxt.Flag_shared = *flags.Shared || *flags.Dynlink
	ctxt.Flag_maymorestack = flags.DebugFlags.MayMoreStack
	ctxt.Debugpcln = flags.DebugFlags.PCTab
	ctxt.IsAsm = true
	ctxt.Pkgpath = *flags.Importpath
	ctxt.Std = *flags.Std
	ctxt.DwTextCount = objabi.DummyDwarfFunctionCountForAssembler()
	switch *flags.Spectre {
	default:
		log.Printf("unknown setting -spectre=%s", *flags.Spectre)
		os.Exit(2)
	case "":
		// nothing
	case "index":
		// known to compiler; ignore here so people can use
		// the same list with -gcflags=-spectre=LIST and -asmflags=-spectre=LIST
	case "all", "ret":
		ctxt.Retpoline = true
	}

	ctxt.Bso = bufio.NewWriter(os.Stdout)
	defer ctxt.Bso.Flush()

	architecture.Init(ctxt)

	// Create object file, write header.
	buf, err := bio.Create(*flags.OutputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer buf.Close()

	if !*flags.SymABIs {
		buf.WriteString(objabi.HeaderString())
		fmt.Fprintf(buf, "!\n")
	}

	// Set macros for GOEXPERIMENTs so we can easily switch
	// runtime assembly code based on them.
	if objabi.LookupPkgSpecial(ctxt.Pkgpath).AllowAsmABI {
		for _, exp := range buildcfg.Experiment.Enabled() {
			flags.D = append(flags.D, "GOEXPERIMENT_"+exp)
		}
	}

	var ok, diag bool
	var failedFile string
	for _, f := range flag.Args() {
		lexer := lex.NewLexer(f)
		parser := asm.NewParser(ctxt, architecture, lexer)
		ctxt.DiagFunc = func(format string, args ...any) {
			diag = true
			log.Printf(format, args...)
		}
		if *flags.SymABIs {
			ok = parser.ParseSymABIs(buf)
		} else {
			pList := new(obj.Plist)
			pList.Firstpc, ok = parser.Parse()
			// reports errors to parser.Errorf
			if ok {
				obj.Flushplist(ctxt, pList, nil)
			}
		}
		if !ok {
			failedFile = f
			break
		}
	}
	if ok && !*flags.SymABIs {
		ctxt.NumberSyms()
		obj.WriteObjFile(ctxt, buf)
	}
	if !ok || diag {
		if failedFile != "" {
			log.Printf("assembly of %s failed", failedFile)
		} else {
			log.Print("assembly failed")
		}
		buf.Close()
		os.Remove(*flags.OutputFile)
		os.Exit(1)
	}
}

```

