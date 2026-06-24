# Domain Architecture: html

## Layout Topology
```text
html/
├── template
│   ├── attr.go
│   ├── attr_string.go
│   ├── content.go
│   ├── context.go
│   ├── css.go
│   ├── delim_string.go
│   ├── doc.go
│   ├── element_string.go
│   ├── error.go
│   ├── escape.go
│   ├── html.go
│   ├── js.go
│   ├── jsctx_string.go
│   ├── state_string.go
│   ├── template.go
│   ├── transition.go
│   ├── url.go
│   └── urlpart_string.go
├── entity.go
└── escape.go
```

## Source Stream Aggregation

// === FILE: references/go/src/html/entity.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package html

import "sync"

// All entities that do not end with ';' are 6 or fewer bytes long.
const longestEntityWithoutSemicolon = 6

// entityMaps returns entity and entity2.
//
// entity is a map from HTML entity names to their values. The semicolon matters:
// https://html.spec.whatwg.org/multipage/named-characters.html
// lists both "amp" and "amp;" as two separate entries.
// Note that the HTML5 list is larger than the HTML4 list at
// http://www.w3.org/TR/html4/sgml/entities.html
//
// entity2 is a map of HTML entities to two unicode codepoints.
var entityMaps = sync.OnceValues(func() (entity map[string]rune, entity2 map[string][2]rune) {
	entity = map[string]rune{
		"AElig;":                           '\U000000C6',
		"AMP;":                             '\U00000026',
		"Aacute;":                          '\U000000C1',
		"Abreve;":                          '\U00000102',
		"Acirc;":                           '\U000000C2',
		"Acy;":                             '\U00000410',
		"Afr;":                             '\U0001D504',
		"Agrave;":                          '\U000000C0',
		"Alpha;":                           '\U00000391',
		"Amacr;":                           '\U00000100',
		"And;":                             '\U00002A53',
		"Aogon;":                           '\U00000104',
		"Aopf;":                            '\U0001D538',
		"ApplyFunction;":                   '\U00002061',
		"Aring;":                           '\U000000C5',
		"Ascr;":                            '\U0001D49C',
		"Assign;":                          '\U00002254',
		"Atilde;":                          '\U000000C3',
		"Auml;":                            '\U000000C4',
		"Backslash;":                       '\U00002216',
		"Barv;":                            '\U00002AE7',
		"Barwed;":                          '\U00002306',
		"Bcy;":                             '\U00000411',
		"Because;":                         '\U00002235',
		"Bernoullis;":                      '\U0000212C',
		"Beta;":                            '\U00000392',
		"Bfr;":                             '\U0001D505',
		"Bopf;":                            '\U0001D539',
		"Breve;":                           '\U000002D8',
		"Bscr;":                            '\U0000212C',
		"Bumpeq;":                          '\U0000224E',
		"CHcy;":                            '\U00000427',
		"COPY;":                            '\U000000A9',
		"Cacute;":                          '\U00000106',
		"Cap;":                             '\U000022D2',
		"CapitalDifferentialD;":            '\U00002145',
		"Cayleys;":                         '\U0000212D',
		"Ccaron;":                          '\U0000010C',
		"Ccedil;":                          '\U000000C7',
		"Ccirc;":                           '\U00000108',
		"Cconint;":                         '\U00002230',
		"Cdot;":                            '\U0000010A',
		"Cedilla;":                         '\U000000B8',
		"CenterDot;":                       '\U000000B7',
		"Cfr;":                             '\U0000212D',
		"Chi;":                             '\U000003A7',
		"CircleDot;":                       '\U00002299',
		"CircleMinus;":                     '\U00002296',
		"CirclePlus;":                      '\U00002295',
		"CircleTimes;":                     '\U00002297',
		"ClockwiseContourIntegral;":        '\U00002232',
		"CloseCurlyDoubleQuote;":           '\U0000201D',
		"CloseCurlyQuote;":                 '\U00002019',
		"Colon;":                           '\U00002237',
		"Colone;":                          '\U00002A74',
		"Congruent;":                       '\U00002261',
		"Conint;":                          '\U0000222F',
		"ContourIntegral;":                 '\U0000222E',
		"Copf;":                            '\U00002102',
		"Coproduct;":                       '\U00002210',
		"CounterClockwiseContourIntegral;": '\U00002233',
		"Cross;":                           '\U00002A2F',
		"Cscr;":                            '\U0001D49E',
		"Cup;":                             '\U000022D3',
		"CupCap;":                          '\U0000224D',
		"DD;":                              '\U00002145',
		"DDotrahd;":                        '\U00002911',
		"DJcy;":                            '\U00000402',
		"DScy;":                            '\U00000405',
		"DZcy;":                            '\U0000040F',
		"Dagger;":                          '\U00002021',
		"Darr;":                            '\U000021A1',
		"Dashv;":                           '\U00002AE4',
		"Dcaron;":                          '\U0000010E',
		"Dcy;":                             '\U00000414',
		"Del;":                             '\U00002207',
		"Delta;":                           '\U00000394',
		"Dfr;":                             '\U0001D507',
		"DiacriticalAcute;":                '\U000000B4',
		"DiacriticalDot;":                  '\U000002D9',
		"DiacriticalDoubleAcute;":          '\U000002DD',
		"DiacriticalGrave;":                '\U00000060',
		"DiacriticalTilde;":                '\U000002DC',
		"Diamond;":                         '\U000022C4',
		"DifferentialD;":                   '\U00002146',
		"Dopf;":                            '\U0001D53B',
		"Dot;":                             '\U000000A8',
		"DotDot;":                          '\U000020DC',
		"DotEqual;":                        '\U00002250',
		"DoubleContourIntegral;":           '\U0000222F',
		"DoubleDot;":                       '\U000000A8',
		"DoubleDownArrow;":                 '\U000021D3',
		"DoubleLeftArrow;":                 '\U000021D0',
		"DoubleLeftRightArrow;":            '\U000021D4',
		"DoubleLeftTee;":                   '\U00002AE4',
		"DoubleLongLeftArrow;":             '\U000027F8',
		"DoubleLongLeftRightArrow;":        '\U000027FA',
		"DoubleLongRightArrow;":            '\U000027F9',
		"DoubleRightArrow;":                '\U000021D2',
		"DoubleRightTee;":                  '\U000022A8',
		"DoubleUpArrow;":                   '\U000021D1',
		"DoubleUpDownArrow;":               '\U000021D5',
		"DoubleVerticalBar;":               '\U00002225',
		"DownArrow;":                       '\U00002193',
		"DownArrowBar;":                    '\U00002913',
		"DownArrowUpArrow;":                '\U000021F5',
		"DownBreve;":                       '\U00000311',
		"DownLeftRightVector;":             '\U00002950',
		"DownLeftTeeVector;":               '\U0000295E',
		"DownLeftVector;":                  '\U000021BD',
		"DownLeftVectorBar;":               '\U00002956',
		"DownRightTeeVector;":              '\U0000295F',
		"DownRightVector;":                 '\U000021C1',
		"DownRightVectorBar;":              '\U00002957',
		"DownTee;":                         '\U000022A4',
		"DownTeeArrow;":                    '\U000021A7',
		"Downarrow;":                       '\U000021D3',
		"Dscr;":                            '\U0001D49F',
		"Dstrok;":                          '\U00000110',
		"ENG;":                             '\U0000014A',
		"ETH;":                             '\U000000D0',
		"Eacute;":                          '\U000000C9',
		"Ecaron;":                          '\U0000011A',
		"Ecirc;":                           '\U000000CA',
		"Ecy;":                             '\U0000042D',
		"Edot;":                            '\U00000116',
		"Efr;":                             '\U0001D508',
		"Egrave;":                          '\U000000C8',
		"Element;":                         '\U00002208',
		"Emacr;":                           '\U00000112',
		"EmptySmallSquare;":                '\U000025FB',
		"EmptyVerySmallSquare;":            '\U000025AB',
		"Eogon;":                           '\U00000118',
		"Eopf;":                            '\U0001D53C',
		"Epsilon;":                         '\U00000395',
		"Equal;":                           '\U00002A75',
		"EqualTilde;":                      '\U00002242',
		"Equilibrium;":                     '\U000021CC',
		"Escr;":                            '\U00002130',
		"Esim;":                            '\U00002A73',
		"Eta;":                             '\U00000397',
		"Euml;":                            '\U000000CB',
		"Exists;":                          '\U00002203',
		"ExponentialE;":                    '\U00002147',
		"Fcy;":                             '\U00000424',
		"Ffr;":                             '\U0001D509',
		"FilledSmallSquare;":               '\U000025FC',
		"FilledVerySmallSquare;":           '\U000025AA',
		"Fopf;":                            '\U0001D53D',
		"ForAll;":                          '\U00002200',
		"Fouriertrf;":                      '\U00002131',
		"Fscr;":                            '\U00002131',
		"GJcy;":                            '\U00000403',
		"GT;":                              '\U0000003E',
		"Gamma;":                           '\U00000393',
		"Gammad;":                          '\U000003DC',
		"Gbreve;":                          '\U0000011E',
		"Gcedil;":                          '\U00000122',
		"Gcirc;":                           '\U0000011C',
		"Gcy;":                             '\U00000413',
		"Gdot;":                            '\U00000120',
		"Gfr;":                             '\U0001D50A',
		"Gg;":                              '\U000022D9',
		"Gopf;":                            '\U0001D53E',
		"GreaterEqual;":                    '\U00002265',
		"GreaterEqualLess;":                '\U000022DB',
		"GreaterFullEqual;":                '\U00002267',
		"GreaterGreater;":                  '\U00002AA2',
		"GreaterLess;":                     '\U00002277',
		"GreaterSlantEqual;":               '\U00002A7E',
		"GreaterTilde;":                    '\U00002273',
		"Gscr;":                            '\U0001D4A2',
		"Gt;":                              '\U0000226B',
		"HARDcy;":                          '\U0000042A',
		"Hacek;":                           '\U000002C7',
		"Hat;":                             '\U0000005E',
		"Hcirc;":                           '\U00000124',
		"Hfr;":                             '\U0000210C',
		"HilbertSpace;":                    '\U0000210B',
		"Hopf;":                            '\U0000210D',
		"HorizontalLine;":                  '\U00002500',
		"Hscr;":                            '\U0000210B',
		"Hstrok;":                          '\U00000126',
		"HumpDownHump;":                    '\U0000224E',
		"HumpEqual;":                       '\U0000224F',
		"IEcy;":                            '\U00000415',
		"IJlig;":                           '\U00000132',
		"IOcy;":                            '\U00000401',
		"Iacute;":                          '\U000000CD',
		"Icirc;":                           '\U000000CE',
		"Icy;":                             '\U00000418',
		"Idot;":                            '\U00000130',
		"Ifr;":                             '\U00002111',
		"Igrave;":                          '\U000000CC',
		"Im;":                              '\U00002111',
		"Imacr;":                           '\U0000012A',
		"ImaginaryI;":                      '\U00002148',
		"Implies;":                         '\U000021D2',
		"Int;":                             '\U0000222C',
		"Integral;":                        '\U0000222B',
		"Intersection;":                    '\U000022C2',
		"InvisibleComma;":                  '\U00002063',
		"InvisibleTimes;":                  '\U00002062',
		"Iogon;":                           '\U0000012E',
		"Iopf;":                            '\U0001D540',
		"Iota;":                            '\U00000399',
		"Iscr;":                            '\U00002110',
		"Itilde;":                          '\U00000128',
		"Iukcy;":                           '\U00000406',
		"Iuml;":                            '\U000000CF',
		"Jcirc;":                           '\U00000134',
		"Jcy;":                             '\U00000419',
		"Jfr;":                             '\U0001D50D',
		"Jopf;":                            '\U0001D541',
		"Jscr;":                            '\U0001D4A5',
		"Jsercy;":                          '\U00000408',
		"Jukcy;":                           '\U00000404',
		"KHcy;":                            '\U00000425',
		"KJcy;":                            '\U0000040C',
		"Kappa;":                           '\U0000039A',
		"Kcedil;":                          '\U00000136',
		"Kcy;":                             '\U0000041A',
		"Kfr;":                             '\U0001D50E',
		"Kopf;":                            '\U0001D542',
		"Kscr;":                            '\U0001D4A6',
		"LJcy;":                            '\U00000409',
		"LT;":                              '\U0000003C',
		"Lacute;":                          '\U00000139',
		"Lambda;":                          '\U0000039B',
		"Lang;":                            '\U000027EA',
		"Laplacetrf;":                      '\U00002112',
		"Larr;":                            '\U0000219E',
		"Lcaron;":                          '\U0000013D',
		"Lcedil;":                          '\U0000013B',
		"Lcy;":                             '\U0000041B',
		"LeftAngleBracket;":                '\U000027E8',
		"LeftArrow;":                       '\U00002190',
		"LeftArrowBar;":                    '\U000021E4',
		"LeftArrowRightArrow;":             '\U000021C6',
		"LeftCeiling;":                     '\U00002308',
		"LeftDoubleBracket;":               '\U000027E6',
		"LeftDownTeeVector;":               '\U00002961',
		"LeftDownVector;":                  '\U000021C3',
		"LeftDownVectorBar;":               '\U00002959',
		"LeftFloor;":                       '\U0000230A',
		"LeftRightArrow;":                  '\U00002194',
		"LeftRightVector;":                 '\U0000294E',
		"LeftTee;":                         '\U000022A3',
		"LeftTeeArrow;":                    '\U000021A4',
		"LeftTeeVector;":                   '\U0000295A',
		"LeftTriangle;":                    '\U000022B2',
		"LeftTriangleBar;":                 '\U000029CF',
		"LeftTriangleEqual;":               '\U000022B4',
		"LeftUpDownVector;":                '\U00002951',
		"LeftUpTeeVector;":                 '\U00002960',
		"LeftUpVector;":                    '\U000021BF',
		"LeftUpVectorBar;":                 '\U00002958',
		"LeftVector;":                      '\U000021BC',
		"LeftVectorBar;":                   '\U00002952',
		"Leftarrow;":                       '\U000021D0',
		"Leftrightarrow;":                  '\U000021D4',
		"LessEqualGreater;":                '\U000022DA',
		"LessFullEqual;":                   '\U00002266',
		"LessGreater;":                     '\U00002276',
		"LessLess;":                        '\U00002AA1',
		"LessSlantEqual;":                  '\U00002A7D',
		"LessTilde;":                       '\U00002272',
		"Lfr;":                             '\U0001D50F',
		"Ll;":                              '\U000022D8',
		"Lleftarrow;":                      '\U000021DA',
		"Lmidot;":                          '\U0000013F',
		"LongLeftArrow;":                   '\U000027F5',
		"LongLeftRightArrow;":              '\U000027F7',
		"LongRightArrow;":                  '\U000027F6',
		"Longleftarrow;":                   '\U000027F8',
		"Longleftrightarrow;":              '\U000027FA',
		"Longrightarrow;":                  '\U000027F9',
		"Lopf;":                            '\U0001D543',
		"LowerLeftArrow;":                  '\U00002199',
		"LowerRightArrow;":                 '\U00002198',
		"Lscr;":                            '\U00002112',
		"Lsh;":                             '\U000021B0',
		"Lstrok;":                          '\U00000141',
		"Lt;":                              '\U0000226A',
		"Map;":                             '\U00002905',
		"Mcy;":                             '\U0000041C',
		"MediumSpace;":                     '\U0000205F',
		"Mellintrf;":                       '\U00002133',
		"Mfr;":                             '\U0001D510',
		"MinusPlus;":                       '\U00002213',
		"Mopf;":                            '\U0001D544',
		"Mscr;":                            '\U00002133',
		"Mu;":                              '\U0000039C',
		"NJcy;":                            '\U0000040A',
		"Nacute;":                          '\U00000143',
		"Ncaron;":                          '\U00000147',
		"Ncedil;":                          '\U00000145',
		"Ncy;":                             '\U0000041D',
		"NegativeMediumSpace;":             '\U0000200B',
		"NegativeThickSpace;":              '\U0000200B',
		"NegativeThinSpace;":               '\U0000200B',
		"NegativeVeryThinSpace;":           '\U0000200B',
		"NestedGreaterGreater;":            '\U0000226B',
		"NestedLessLess;":                  '\U0000226A',
		"NewLine;":                         '\U0000000A',
		"Nfr;":                             '\U0001D511',
		"NoBreak;":                         '\U00002060',
		"NonBreakingSpace;":                '\U000000A0',
		"Nopf;":                            '\U00002115',
		"Not;":                             '\U00002AEC',
		"NotCongruent;":                    '\U00002262',
		"NotCupCap;":                       '\U0000226D',
		"NotDoubleVerticalBar;":            '\U00002226',
		"NotElement;":                      '\U00002209',
		"NotEqual;":                        '\U00002260',
		"NotExists;":                       '\U00002204',
		"NotGreater;":                      '\U0000226F',
		"NotGreaterEqual;":                 '\U00002271',
		"NotGreaterLess;":                  '\U00002279',
		"NotGreaterTilde;":                 '\U00002275',
		"NotLeftTriangle;":                 '\U000022EA',
		"NotLeftTriangleEqual;":            '\U000022EC',
		"NotLess;":                         '\U0000226E',
		"NotLessEqual;":                    '\U00002270',
		"NotLessGreater;":                  '\U00002278',
		"NotLessTilde;":                    '\U00002274',
		"NotPrecedes;":                     '\U00002280',
		"NotPrecedesSlantEqual;":           '\U000022E0',
		"NotReverseElement;":               '\U0000220C',
		"NotRightTriangle;":                '\U000022EB',
		"NotRightTriangleEqual;":           '\U000022ED',
		"NotSquareSubsetEqual;":            '\U000022E2',
		"NotSquareSupersetEqual;":          '\U000022E3',
		"NotSubsetEqual;":                  '\U00002288',
		"NotSucceeds;":                     '\U00002281',
		"NotSucceedsSlantEqual;":           '\U000022E1',
		"NotSupersetEqual;":                '\U00002289',
		"NotTilde;":                        '\U00002241',
		"NotTildeEqual;":                   '\U00002244',
		"NotTildeFullEqual;":               '\U00002247',
		"NotTildeTilde;":                   '\U00002249',
		"NotVerticalBar;":                  '\U00002224',
		"Nscr;":                            '\U0001D4A9',
		"Ntilde;":                          '\U000000D1',
		"Nu;":                              '\U0000039D',
		"OElig;":                           '\U00000152',
		"Oacute;":                          '\U000000D3',
		"Ocirc;":                           '\U000000D4',
		"Ocy;":                             '\U0000041E',
		"Odblac;":                          '\U00000150',
		"Ofr;":                             '\U0001D512',
		"Ograve;":                          '\U000000D2',
		"Omacr;":                           '\U0000014C',
		"Omega;":                           '\U000003A9',
		"Omicron;":                         '\U0000039F',
		"Oopf;":                            '\U0001D546',
		"OpenCurlyDoubleQuote;":            '\U0000201C',
		"OpenCurlyQuote;":                  '\U00002018',
		"Or;":                              '\U00002A54',
		"Oscr;":                            '\U0001D4AA',
		"Oslash;":                          '\U000000D8',
		"Otilde;":                          '\U000000D5',
		"Otimes;":                          '\U00002A37',
		"Ouml;":                            '\U000000D6',
		"OverBar;":                         '\U0000203E',
		"OverBrace;":                       '\U000023DE',
		"OverBracket;":                     '\U000023B4',
		"OverParenthesis;":                 '\U000023DC',
		"PartialD;":                        '\U00002202',
		"Pcy;":                             '\U0000041F',
		"Pfr;":                             '\U0001D513',
		"Phi;":                             '\U000003A6',
		"Pi;":                              '\U000003A0',
		"PlusMinus;":                       '\U000000B1',
		"Poincareplane;":                   '\U0000210C',
		"Popf;":                            '\U00002119',
		"Pr;":                              '\U00002ABB',
		"Precedes;":                        '\U0000227A',
		"PrecedesEqual;":                   '\U00002AAF',
		"PrecedesSlantEqual;":              '\U0000227C',
		"PrecedesTilde;":                   '\U0000227E',
		"Prime;":                           '\U00002033',
		"Product;":                         '\U0000220F',
		"Proportion;":                      '\U00002237',
		"Proportional;":                    '\U0000221D',
		"Pscr;":                            '\U0001D4AB',
		"Psi;":                             '\U000003A8',
		"QUOT;":                            '\U00000022',
		"Qfr;":                             '\U0001D514',
		"Qopf;":                            '\U0000211A',
		"Qscr;":                            '\U0001D4AC',
		"RBarr;":                           '\U00002910',
		"REG;":                             '\U000000AE',
		"Racute;":                          '\U00000154',
		"Rang;":                            '\U000027EB',
		"Rarr;":                            '\U000021A0',
		"Rarrtl;":                          '\U00002916',
		"Rcaron;":                          '\U00000158',
		"Rcedil;":                          '\U00000156',
		"Rcy;":                             '\U00000420',
		"Re;":                              '\U0000211C',
		"ReverseElement;":                  '\U0000220B',
		"ReverseEquilibrium;":              '\U000021CB',
		"ReverseUpEquilibrium;":            '\U0000296F',
		"Rfr;":                             '\U0000211C',
		"Rho;":                             '\U000003A1',
		"RightAngleBracket;":               '\U000027E9',
		"RightArrow;":                      '\U00002192',
		"RightArrowBar;":                   '\U000021E5',
		"RightArrowLeftArrow;":             '\U000021C4',
		"RightCeiling;":                    '\U00002309',
		"RightDoubleBracket;":              '\U000027E7',
		"RightDownTeeVector;":              '\U0000295D',
		"RightDownVector;":                 '\U000021C2',
		"RightDownVectorBar;":              '\U00002955',
		"RightFloor;":                      '\U0000230B',
		"RightTee;":                        '\U000022A2',
		"RightTeeArrow;":                   '\U000021A6',
		"RightTeeVector;":                  '\U0000295B',
		"RightTriangle;":                   '\U000022B3',
		"RightTriangleBar;":                '\U000029D0',
		"RightTriangleEqual;":              '\U000022B5',
		"RightUpDownVector;":               '\U0000294F',
		"RightUpTeeVector;":                '\U0000295C',
		"RightUpVector;":                   '\U000021BE',
		"RightUpVectorBar;":                '\U00002954',
		"RightVector;":                     '\U000021C0',
		"RightVectorBar;":                  '\U00002953',
		"Rightarrow;":                      '\U000021D2',
		"Ropf;":                            '\U0000211D',
		"RoundImplies;":                    '\U00002970',
		"Rrightarrow;":                     '\U000021DB',
		"Rscr;":                            '\U0000211B',
		"Rsh;":                             '\U000021B1',
		"RuleDelayed;":                     '\U000029F4',
		"SHCHcy;":                          '\U00000429',
		"SHcy;":                            '\U00000428',
		"SOFTcy;":                          '\U0000042C',
		"Sacute;":                          '\U0000015A',
		"Sc;":                              '\U00002ABC',
		"Scaron;":                          '\U00000160',
		"Scedil;":                          '\U0000015E',
		"Scirc;":                           '\U0000015C',
		"Scy;":                             '\U00000421',
		"Sfr;":                             '\U0001D516',
		"ShortDownArrow;":                  '\U00002193',
		"ShortLeftArrow;":                  '\U00002190',
		"ShortRightArrow;":                 '\U00002192',
		"ShortUpArrow;":                    '\U00002191',
		"Sigma;":                           '\U000003A3',
		"SmallCircle;":                     '\U00002218',
		"Sopf;":                            '\U0001D54A',
		"Sqrt;":                            '\U0000221A',
		"Square;":                          '\U000025A1',
		"SquareIntersection;":              '\U00002293',
		"SquareSubset;":                    '\U0000228F',
		"SquareSubsetEqual;":               '\U00002291',
		"SquareSuperset;":                  '\U00002290',
		"SquareSupersetEqual;":             '\U00002292',
		"SquareUnion;":                     '\U00002294',
		"Sscr;":                            '\U0001D4AE',
		"Star;":                            '\U000022C6',
		"Sub;":                             '\U000022D0',
		"Subset;":                          '\U000022D0',
		"SubsetEqual;":                     '\U00002286',
		"Succeeds;":                        '\U0000227B',
		"SucceedsEqual;":                   '\U00002AB0',
		"SucceedsSlantEqual;":              '\U0000227D',
		"SucceedsTilde;":                   '\U0000227F',
		"SuchThat;":                        '\U0000220B',
		"Sum;":                             '\U00002211',
		"Sup;":                             '\U000022D1',
		"Superset;":                        '\U00002283',
		"SupersetEqual;":                   '\U00002287',
		"Supset;":                          '\U000022D1',
		"THORN;":                           '\U000000DE',
		"TRADE;":                           '\U00002122',
		"TSHcy;":                           '\U0000040B',
		"TScy;":                            '\U00000426',
		"Tab;":                             '\U00000009',
		"Tau;":                             '\U000003A4',
		"Tcaron;":                          '\U00000164',
		"Tcedil;":                          '\U00000162',
		"Tcy;":                             '\U00000422',
		"Tfr;":                             '\U0001D517',
		"Therefore;":                       '\U00002234',
		"Theta;":                           '\U00000398',
		"ThinSpace;":                       '\U00002009',
		"Tilde;":                           '\U0000223C',
		"TildeEqual;":                      '\U00002243',
		"TildeFullEqual;":                  '\U00002245',
		"TildeTilde;":                      '\U00002248',
		"Topf;":                            '\U0001D54B',
		"TripleDot;":                       '\U000020DB',
		"Tscr;":                            '\U0001D4AF',
		"Tstrok;":                          '\U00000166',
		"Uacute;":                          '\U000000DA',
		"Uarr;":                            '\U0000219F',
		"Uarrocir;":                        '\U00002949',
		"Ubrcy;":                           '\U0000040E',
		"Ubreve;":                          '\U0000016C',
		"Ucirc;":                           '\U000000DB',
		"Ucy;":                             '\U00000423',
		"Udblac;":                          '\U00000170',
		"Ufr;":                             '\U0001D518',
		"Ugrave;":                          '\U000000D9',
		"Umacr;":                           '\U0000016A',
		"UnderBar;":                        '\U0000005F',
		"UnderBrace;":                      '\U000023DF',
		"UnderBracket;":                    '\U000023B5',
		"UnderParenthesis;":                '\U000023DD',
		"Union;":                           '\U000022C3',
		"UnionPlus;":                       '\U0000228E',
		"Uogon;":                           '\U00000172',
		"Uopf;":                            '\U0001D54C',
		"UpArrow;":                         '\U00002191',
		"UpArrowBar;":                      '\U00002912',
		"UpArrowDownArrow;":                '\U000021C5',
		"UpDownArrow;":                     '\U00002195',
		"UpEquilibrium;":                   '\U0000296E',
		"UpTee;":                           '\U000022A5',
		"UpTeeArrow;":                      '\U000021A5',
		"Uparrow;":                         '\U000021D1',
		"Updownarrow;":                     '\U000021D5',
		"UpperLeftArrow;":                  '\U00002196',
		"UpperRightArrow;":                 '\U00002197',
		"Upsi;":                            '\U000003D2',
		"Upsilon;":                         '\U000003A5',
		"Uring;":                           '\U0000016E',
		"Uscr;":                            '\U0001D4B0',
		"Utilde;":                          '\U00000168',
		"Uuml;":                            '\U000000DC',
		"VDash;":                           '\U000022AB',
		"Vbar;":                            '\U00002AEB',
		"Vcy;":                             '\U00000412',
		"Vdash;":                           '\U000022A9',
		"Vdashl;":                          '\U00002AE6',
		"Vee;":                             '\U000022C1',
		"Verbar;":                          '\U00002016',
		"Vert;":                            '\U00002016',
		"VerticalBar;":                     '\U00002223',
		"VerticalLine;":                    '\U0000007C',
		"VerticalSeparator;":               '\U00002758',
		"VerticalTilde;":                   '\U00002240',
		"VeryThinSpace;":                   '\U0000200A',
		"Vfr;":                             '\U0001D519',
		"Vopf;":                            '\U0001D54D',
		"Vscr;":                            '\U0001D4B1',
		"Vvdash;":                          '\U000022AA',
		"Wcirc;":                           '\U00000174',
		"Wedge;":                           '\U000022C0',
		"Wfr;":                             '\U0001D51A',
		"Wopf;":                            '\U0001D54E',
		"Wscr;":                            '\U0001D4B2',
		"Xfr;":                             '\U0001D51B',
		"Xi;":                              '\U0000039E',
		"Xopf;":                            '\U0001D54F',
		"Xscr;":                            '\U0001D4B3',
		"YAcy;":                            '\U0000042F',
		"YIcy;":                            '\U00000407',
		"YUcy;":                            '\U0000042E',
		"Yacute;":                          '\U000000DD',
		"Ycirc;":                           '\U00000176',
		"Ycy;":                             '\U0000042B',
		"Yfr;":                             '\U0001D51C',
		"Yopf;":                            '\U0001D550',
		"Yscr;":                            '\U0001D4B4',
		"Yuml;":                            '\U00000178',
		"ZHcy;":                            '\U00000416',
		"Zacute;":                          '\U00000179',
		"Zcaron;":                          '\U0000017D',
		"Zcy;":                             '\U00000417',
		"Zdot;":                            '\U0000017B',
		"ZeroWidthSpace;":                  '\U0000200B',
		"Zeta;":                            '\U00000396',
		"Zfr;":                             '\U00002128',
		"Zopf;":                            '\U00002124',
		"Zscr;":                            '\U0001D4B5',
		"aacute;":                          '\U000000E1',
		"abreve;":                          '\U00000103',
		"ac;":                              '\U0000223E',
		"acd;":                             '\U0000223F',
		"acirc;":                           '\U000000E2',
		"acute;":                           '\U000000B4',
		"acy;":                             '\U00000430',
		"aelig;":                           '\U000000E6',
		"af;":                              '\U00002061',
		"afr;":                             '\U0001D51E',
		"agrave;":                          '\U000000E0',
		"alefsym;":                         '\U00002135',
		"aleph;":                           '\U00002135',
		"alpha;":                           '\U000003B1',
		"amacr;":                           '\U00000101',
		"amalg;":                           '\U00002A3F',
		"amp;":                             '\U00000026',
		"and;":                             '\U00002227',
		"andand;":                          '\U00002A55',
		"andd;":                            '\U00002A5C',
		"andslope;":                        '\U00002A58',
		"andv;":                            '\U00002A5A',
		"ang;":                             '\U00002220',
		"ange;":                            '\U000029A4',
		"angle;":                           '\U00002220',
		"angmsd;":                          '\U00002221',
		"angmsdaa;":                        '\U000029A8',
		"angmsdab;":                        '\U000029A9',
		"angmsdac;":                        '\U000029AA',
		"angmsdad;":                        '\U000029AB',
		"angmsdae;":                        '\U000029AC',
		"angmsdaf;":                        '\U000029AD',
		"angmsdag;":                        '\U000029AE',
		"angmsdah;":                        '\U000029AF',
		"angrt;":                           '\U0000221F',
		"angrtvb;":                         '\U000022BE',
		"angrtvbd;":                        '\U0000299D',
		"angsph;":                          '\U00002222',
		"angst;":                           '\U000000C5',
		"angzarr;":                         '\U0000237C',
		"aogon;":                           '\U00000105',
		"aopf;":                            '\U0001D552',
		"ap;":                              '\U00002248',
		"apE;":                             '\U00002A70',
		"apacir;":                          '\U00002A6F',
		"ape;":                             '\U0000224A',
		"apid;":                            '\U0000224B',
		"apos;":                            '\U00000027',
		"approx;":                          '\U00002248',
		"approxeq;":                        '\U0000224A',
		"aring;":                           '\U000000E5',
		"ascr;":                            '\U0001D4B6',
		"ast;":                             '\U0000002A',
		"asymp;":                           '\U00002248',
		"asympeq;":                         '\U0000224D',
		"atilde;":                          '\U000000E3',
		"auml;":                            '\U000000E4',
		"awconint;":                        '\U00002233',
		"awint;":                           '\U00002A11',
		"bNot;":                            '\U00002AED',
		"backcong;":                        '\U0000224C',
		"backepsilon;":                     '\U000003F6',
		"backprime;":                       '\U00002035',
		"backsim;":                         '\U0000223D',
		"backsimeq;":                       '\U000022CD',
		"barvee;":                          '\U000022BD',
		"barwed;":                          '\U00002305',
		"barwedge;":                        '\U00002305',
		"bbrk;":                            '\U000023B5',
		"bbrktbrk;":                        '\U000023B6',
		"bcong;":                           '\U0000224C',
		"bcy;":                             '\U00000431',
		"bdquo;":                           '\U0000201E',
		"becaus;":                          '\U00002235',
		"because;":                         '\U00002235',
		"bemptyv;":                         '\U000029B0',
		"bepsi;":                           '\U000003F6',
		"bernou;":                          '\U0000212C',
		"beta;":                            '\U000003B2',
		"beth;":                            '\U00002136',
		"between;":                         '\U0000226C',
		"bfr;":                             '\U0001D51F',
		"bigcap;":                          '\U000022C2',
		"bigcirc;":                         '\U000025EF',
		"bigcup;":                          '\U000022C3',
		"bigodot;":                         '\U00002A00',
		"bigoplus;":                        '\U00002A01',
		"bigotimes;":                       '\U00002A02',
		"bigsqcup;":                        '\U00002A06',
		"bigstar;":                         '\U00002605',
		"bigtriangledown;":                 '\U000025BD',
		"bigtriangleup;":                   '\U000025B3',
		"biguplus;":                        '\U00002A04',
		"bigvee;":                          '\U000022C1',
		"bigwedge;":                        '\U000022C0',
		"bkarow;":                          '\U0000290D',
		"blacklozenge;":                    '\U000029EB',
		"blacksquare;":                     '\U000025AA',
		"blacktriangle;":                   '\U000025B4',
		"blacktriangledown;":               '\U000025BE',
		"blacktriangleleft;":               '\U000025C2',
		"blacktriangleright;":              '\U000025B8',
		"blank;":                           '\U00002423',
		"blk12;":                           '\U00002592',
		"blk14;":                           '\U00002591',
		"blk34;":                           '\U00002593',
		"block;":                           '\U00002588',
		"bnot;":                            '\U00002310',
		"bopf;":                            '\U0001D553',
		"bot;":                             '\U000022A5',
		"bottom;":                          '\U000022A5',
		"bowtie;":                          '\U000022C8',
		"boxDL;":                           '\U00002557',
		"boxDR;":                           '\U00002554',
		"boxDl;":                           '\U00002556',
		"boxDr;":                           '\U00002553',
		"boxH;":                            '\U00002550',
		"boxHD;":                           '\U00002566',
		"boxHU;":                           '\U00002569',
		"boxHd;":                           '\U00002564',
		"boxHu;":                           '\U00002567',
		"boxUL;":                           '\U0000255D',
		"boxUR;":                           '\U0000255A',
		"boxUl;":                           '\U0000255C',
		"boxUr;":                           '\U00002559',
		"boxV;":                            '\U00002551',
		"boxVH;":                           '\U0000256C',
		"boxVL;":                           '\U00002563',
		"boxVR;":                           '\U00002560',
		"boxVh;":                           '\U0000256B',
		"boxVl;":                           '\U00002562',
		"boxVr;":                           '\U0000255F',
		"boxbox;":                          '\U000029C9',
		"boxdL;":                           '\U00002555',
		"boxdR;":                           '\U00002552',
		"boxdl;":                           '\U00002510',
		"boxdr;":                           '\U0000250C',
		"boxh;":                            '\U00002500',
		"boxhD;":                           '\U00002565',
		"boxhU;":                           '\U00002568',
		"boxhd;":                           '\U0000252C',
		"boxhu;":                           '\U00002534',
		"boxminus;":                        '\U0000229F',
		"boxplus;":                         '\U0000229E',
		"boxtimes;":                        '\U000022A0',
		"boxuL;":                           '\U0000255B',
		"boxuR;":                           '\U00002558',
		"boxul;":                           '\U00002518',
		"boxur;":                           '\U00002514',
		"boxv;":                            '\U00002502',
		"boxvH;":                           '\U0000256A',
		"boxvL;":                           '\U00002561',
		"boxvR;":                           '\U0000255E',
		"boxvh;":                           '\U0000253C',
		"boxvl;":                           '\U00002524',
		"boxvr;":                           '\U0000251C',
		"bprime;":                          '\U00002035',
		"breve;":                           '\U000002D8',
		"brvbar;":                          '\U000000A6',
		"bscr;":                            '\U0001D4B7',
		"bsemi;":                           '\U0000204F',
		"bsim;":                            '\U0000223D',
		"bsime;":                           '\U000022CD',
		"bsol;":                            '\U0000005C',
		"bsolb;":                           '\U000029C5',
		"bsolhsub;":                        '\U000027C8',
		"bull;":                            '\U00002022',
		"bullet;":                          '\U00002022',
		"bump;":                            '\U0000224E',
		"bumpE;":                           '\U00002AAE',
		"bumpe;":                           '\U0000224F',
		"bumpeq;":                          '\U0000224F',
		"cacute;":                          '\U00000107',
		"cap;":                             '\U00002229',
		"capand;":                          '\U00002A44',
		"capbrcup;":                        '\U00002A49',
		"capcap;":                          '\U00002A4B',
		"capcup;":                          '\U00002A47',
		"capdot;":                          '\U00002A40',
		"caret;":                           '\U00002041',
		"caron;":                           '\U000002C7',
		"ccaps;":                           '\U00002A4D',
		"ccaron;":                          '\U0000010D',
		"ccedil;":                          '\U000000E7',
		"ccirc;":                           '\U00000109',
		"ccups;":                           '\U00002A4C',
		"ccupssm;":                         '\U00002A50',
		"cdot;":                            '\U0000010B',
		"cedil;":                           '\U000000B8',
		"cemptyv;":                         '\U000029B2',
		"cent;":                            '\U000000A2',
		"centerdot;":                       '\U000000B7',
		"cfr;":                             '\U0001D520',
		"chcy;":                            '\U00000447',
		"check;":                           '\U00002713',
		"checkmark;":                       '\U00002713',
		"chi;":                             '\U000003C7',
		"cir;":                             '\U000025CB',
		"cirE;":                            '\U000029C3',
		"circ;":                            '\U000002C6',
		"circeq;":                          '\U00002257',
		"circlearrowleft;":                 '\U000021BA',
		"circlearrowright;":                '\U000021BB',
		"circledR;":                        '\U000000AE',
		"circledS;":                        '\U000024C8',
		"circledast;":                      '\U0000229B',
		"circledcirc;":                     '\U0000229A',
		"circleddash;":                     '\U0000229D',
		"cire;":                            '\U00002257',
		"cirfnint;":                        '\U00002A10',
		"cirmid;":                          '\U00002AEF',
		"cirscir;":                         '\U000029C2',
		"clubs;":                           '\U00002663',
		"clubsuit;":                        '\U00002663',
		"colon;":                           '\U0000003A',
		"colone;":                          '\U00002254',
		"coloneq;":                         '\U00002254',
		"comma;":                           '\U0000002C',
		"commat;":                          '\U00000040',
		"comp;":                            '\U00002201',
		"compfn;":                          '\U00002218',
		"complement;":                      '\U00002201',
		"complexes;":                       '\U00002102',
		"cong;":                            '\U00002245',
		"congdot;":                         '\U00002A6D',
		"conint;":                          '\U0000222E',
		"copf;":                            '\U0001D554',
		"coprod;":                          '\U00002210',
		"copy;":                            '\U000000A9',
		"copysr;":                          '\U00002117',
		"crarr;":                           '\U000021B5',
		"cross;":                           '\U00002717',
		"cscr;":                            '\U0001D4B8',
		"csub;":                            '\U00002ACF',
		"csube;":                           '\U00002AD1',
		"csup;":                            '\U00002AD0',
		"csupe;":                           '\U00002AD2',
		"ctdot;":                           '\U000022EF',
		"cudarrl;":                         '\U00002938',
		"cudarrr;":                         '\U00002935',
		"cuepr;":                           '\U000022DE',
		"cuesc;":                           '\U000022DF',
		"cularr;":                          '\U000021B6',
		"cularrp;":                         '\U0000293D',
		"cup;":                             '\U0000222A',
		"cupbrcap;":                        '\U00002A48',
		"cupcap;":                          '\U00002A46',
		"cupcup;":                          '\U00002A4A',
		"cupdot;":                          '\U0000228D',
		"cupor;":                           '\U00002A45',
		"curarr;":                          '\U000021B7',
		"curarrm;":                         '\U0000293C',
		"curlyeqprec;":                     '\U000022DE',
		"curlyeqsucc;":                     '\U000022DF',
		"curlyvee;":                        '\U000022CE',
		"curlywedge;":                      '\U000022CF',
		"curren;":                          '\U000000A4',
		"curvearrowleft;":                  '\U000021B6',
		"curvearrowright;":                 '\U000021B7',
		"cuvee;":                           '\U000022CE',
		"cuwed;":                           '\U000022CF',
		"cwconint;":                        '\U00002232',
		"cwint;":                           '\U00002231',
		"cylcty;":                          '\U0000232D',
		"dArr;":                            '\U000021D3',
		"dHar;":                            '\U00002965',
		"dagger;":                          '\U00002020',
		"daleth;":                          '\U00002138',
		"darr;":                            '\U00002193',
		"dash;":                            '\U00002010',
		"dashv;":                           '\U000022A3',
		"dbkarow;":                         '\U0000290F',
		"dblac;":                           '\U000002DD',
		"dcaron;":                          '\U0000010F',
		"dcy;":                             '\U00000434',
		"dd;":                              '\U00002146',
		"ddagger;":                         '\U00002021',
		"ddarr;":                           '\U000021CA',
		"ddotseq;":                         '\U00002A77',
		"deg;":                             '\U000000B0',
		"delta;":                           '\U000003B4',
		"demptyv;":                         '\U000029B1',
		"dfisht;":                          '\U0000297F',
		"dfr;":                             '\U0001D521',
		"dharl;":                           '\U000021C3',
		"dharr;":                           '\U000021C2',
		"diam;":                            '\U000022C4',
		"diamond;":                         '\U000022C4',
		"diamondsuit;":                     '\U00002666',
		"diams;":                           '\U00002666',
		"die;":                             '\U000000A8',
		"digamma;":                         '\U000003DD',
		"disin;":                           '\U000022F2',
		"div;":                             '\U000000F7',
		"divide;":                          '\U000000F7',
		"divideontimes;":                   '\U000022C7',
		"divonx;":                          '\U000022C7',
		"djcy;":                            '\U00000452',
		"dlcorn;":                          '\U0000231E',
		"dlcrop;":                          '\U0000230D',
		"dollar;":                          '\U00000024',
		"dopf;":                            '\U0001D555',
		"dot;":                             '\U000002D9',
		"doteq;":                           '\U00002250',
		"doteqdot;":                        '\U00002251',
		"dotminus;":                        '\U00002238',
		"dotplus;":                         '\U00002214',
		"dotsquare;":                       '\U000022A1',
		"doublebarwedge;":                  '\U00002306',
		"downarrow;":                       '\U00002193',
		"downdownarrows;":                  '\U000021CA',
		"downharpoonleft;":                 '\U000021C3',
		"downharpoonright;":                '\U000021C2',
		"drbkarow;":                        '\U00002910',
		"drcorn;":                          '\U0000231F',
		"drcrop;":                          '\U0000230C',
		"dscr;":                            '\U0001D4B9',
		"dscy;":                            '\U00000455',
		"dsol;":                            '\U000029F6',
		"dstrok;":                          '\U00000111',
		"dtdot;":                           '\U000022F1',
		"dtri;":                            '\U000025BF',
		"dtrif;":                           '\U000025BE',
		"duarr;":                           '\U000021F5',
		"duhar;":                           '\U0000296F',
		"dwangle;":                         '\U000029A6',
		"dzcy;":                            '\U0000045F',
		"dzigrarr;":                        '\U000027FF',
		"eDDot;":                           '\U00002A77',
		"eDot;":                            '\U00002251',
		"eacute;":                          '\U000000E9',
		"easter;":                          '\U00002A6E',
		"ecaron;":                          '\U0000011B',
		"ecir;":                            '\U00002256',
		"ecirc;":                           '\U000000EA',
		"ecolon;":                          '\U00002255',
		"ecy;":                             '\U0000044D',
		"edot;":                            '\U00000117',
		"ee;":                              '\U00002147',
		"efDot;":                           '\U00002252',
		"efr;":                             '\U0001D522',
		"eg;":                              '\U00002A9A',
		"egrave;":                          '\U000000E8',
		"egs;":                             '\U00002A96',
		"egsdot;":                          '\U00002A98',
		"el;":                              '\U00002A99',
		"elinters;":                        '\U000023E7',
		"ell;":                             '\U00002113',
		"els;":                             '\U00002A95',
		"elsdot;":                          '\U00002A97',
		"emacr;":                           '\U00000113',
		"empty;":                           '\U00002205',
		"emptyset;":                        '\U00002205',
		"emptyv;":                          '\U00002205',
		"emsp;":                            '\U00002003',
		"emsp13;":                          '\U00002004',
		"emsp14;":                          '\U00002005',
		"eng;":                             '\U0000014B',
		"ensp;":                            '\U00002002',
		"eogon;":                           '\U00000119',
		"eopf;":                            '\U0001D556',
		"epar;":                            '\U000022D5',
		"eparsl;":                          '\U000029E3',
		"eplus;":                           '\U00002A71',
		"epsi;":                            '\U000003B5',
		"epsilon;":                         '\U000003B5',
		"epsiv;":                           '\U000003F5',
		"eqcirc;":                          '\U00002256',
		"eqcolon;":                         '\U00002255',
		"eqsim;":                           '\U00002242',
		"eqslantgtr;":                      '\U00002A96',
		"eqslantless;":                     '\U00002A95',
		"equals;":                          '\U0000003D',
		"equest;":                          '\U0000225F',
		"equiv;":                           '\U00002261',
		"equivDD;":                         '\U00002A78',
		"eqvparsl;":                        '\U000029E5',
		"erDot;":                           '\U00002253',
		"erarr;":                           '\U00002971',
		"escr;":                            '\U0000212F',
		"esdot;":                           '\U00002250',
		"esim;":                            '\U00002242',
		"eta;":                             '\U000003B7',
		"eth;":                             '\U000000F0',
		"euml;":                            '\U000000EB',
		"euro;":                            '\U000020AC',
		"excl;":                            '\U00000021',
		"exist;":                           '\U00002203',
		"expectation;":                     '\U00002130',
		"exponentiale;":                    '\U00002147',
		"fallingdotseq;":                   '\U00002252',
		"fcy;":                             '\U00000444',
		"female;":                          '\U00002640',
		"ffilig;":                          '\U0000FB03',
		"fflig;":                           '\U0000FB00',
		"ffllig;":                          '\U0000FB04',
		"ffr;":                             '\U0001D523',
		"filig;":                           '\U0000FB01',
		"flat;":                            '\U0000266D',
		"fllig;":                           '\U0000FB02',
		"fltns;":                           '\U000025B1',
		"fnof;":                            '\U00000192',
		"fopf;":                            '\U0001D557',
		"forall;":                          '\U00002200',
		"fork;":                            '\U000022D4',
		"forkv;":                           '\U00002AD9',
		"fpartint;":                        '\U00002A0D',
		"frac12;":                          '\U000000BD',
		"frac13;":                          '\U00002153',
		"frac14;":                          '\U000000BC',
		"frac15;":                          '\U00002155',
		"frac16;":                          '\U00002159',
		"frac18;":                          '\U0000215B',
		"frac23;":                          '\U00002154',
		"frac25;":                          '\U00002156',
		"frac34;":                          '\U000000BE',
		"frac35;":                          '\U00002157',
		"frac38;":                          '\U0000215C',
		"frac45;":                          '\U00002158',
		"frac56;":                          '\U0000215A',
		"frac58;":                          '\U0000215D',
		"frac78;":                          '\U0000215E',
		"frasl;":                           '\U00002044',
		"frown;":                           '\U00002322',
		"fscr;":                            '\U0001D4BB',
		"gE;":                              '\U00002267',
		"gEl;":                             '\U00002A8C',
		"gacute;":                          '\U000001F5',
		"gamma;":                           '\U000003B3',
		"gammad;":                          '\U000003DD',
		"gap;":                             '\U00002A86',
		"gbreve;":                          '\U0000011F',
		"gcirc;":                           '\U0000011D',
		"gcy;":                             '\U00000433',
		"gdot;":                            '\U00000121',
		"ge;":                              '\U00002265',
		"gel;":                             '\U000022DB',
		"geq;":                             '\U00002265',
		"geqq;":                            '\U00002267',
		"geqslant;":                        '\U00002A7E',
		"ges;":                             '\U00002A7E',
		"gescc;":                           '\U00002AA9',
		"gesdot;":                          '\U00002A80',
		"gesdoto;":                         '\U00002A82',
		"gesdotol;":                        '\U00002A84',
		"gesles;":                          '\U00002A94',
		"gfr;":                             '\U0001D524',
		"gg;":                              '\U0000226B',
		"ggg;":                             '\U000022D9',
		"gimel;":                           '\U00002137',
		"gjcy;":                            '\U00000453',
		"gl;":                              '\U00002277',
		"glE;":                             '\U00002A92',
		"gla;":                             '\U00002AA5',
		"glj;":                             '\U00002AA4',
		"gnE;":                             '\U00002269',
		"gnap;":                            '\U00002A8A',
		"gnapprox;":                        '\U00002A8A',
		"gne;":                             '\U00002A88',
		"gneq;":                            '\U00002A88',
		"gneqq;":                           '\U00002269',
		"gnsim;":                           '\U000022E7',
		"gopf;":                            '\U0001D558',
		"grave;":                           '\U00000060',
		"gscr;":                            '\U0000210A',
		"gsim;":                            '\U00002273',
		"gsime;":                           '\U00002A8E',
		"gsiml;":                           '\U00002A90',
		"gt;":                              '\U0000003E',
		"gtcc;":                            '\U00002AA7',
		"gtcir;":                           '\U00002A7A',
		"gtdot;":                           '\U000022D7',
		"gtlPar;":                          '\U00002995',
		"gtquest;":                         '\U00002A7C',
		"gtrapprox;":                       '\U00002A86',
		"gtrarr;":                          '\U00002978',
		"gtrdot;":                          '\U000022D7',
		"gtreqless;":                       '\U000022DB',
		"gtreqqless;":                      '\U00002A8C',
		"gtrless;":                         '\U00002277',
		"gtrsim;":                          '\U00002273',
		"hArr;":                            '\U000021D4',
		"hairsp;":                          '\U0000200A',
		"half;":                            '\U000000BD',
		"hamilt;":                          '\U0000210B',
		"hardcy;":                          '\U0000044A',
		"harr;":                            '\U00002194',
		"harrcir;":                         '\U00002948',
		"harrw;":                           '\U000021AD',
		"hbar;":                            '\U0000210F',
		"hcirc;":                           '\U00000125',
		"hearts;":                          '\U00002665',
		"heartsuit;":                       '\U00002665',
		"hellip;":                          '\U00002026',
		"hercon;":                          '\U000022B9',
		"hfr;":                             '\U0001D525',
		"hksearow;":                        '\U00002925',
		"hkswarow;":                        '\U00002926',
		"hoarr;":                           '\U000021FF',
		"homtht;":                          '\U0000223B',
		"hookleftarrow;":                   '\U000021A9',
		"hookrightarrow;":                  '\U000021AA',
		"hopf;":                            '\U0001D559',
		"horbar;":                          '\U00002015',
		"hscr;":                            '\U0001D4BD',
		"hslash;":                          '\U0000210F',
		"hstrok;":                          '\U00000127',
		"hybull;":                          '\U00002043',
		"hyphen;":                          '\U00002010',
		"iacute;":                          '\U000000ED',
		"ic;":                              '\U00002063',
		"icirc;":                           '\U000000EE',
		"icy;":                             '\U00000438',
		"iecy;":                            '\U00000435',
		"iexcl;":                           '\U000000A1',
		"iff;":                             '\U000021D4',
		"ifr;":                             '\U0001D526',
		"igrave;":                          '\U000000EC',
		"ii;":                              '\U00002148',
		"iiiint;":                          '\U00002A0C',
		"iiint;":                           '\U0000222D',
		"iinfin;":                          '\U000029DC',
		"iiota;":                           '\U00002129',
		"ijlig;":                           '\U00000133',
		"imacr;":                           '\U0000012B',
		"image;":                           '\U00002111',
		"imagline;":                        '\U00002110',
		"imagpart;":                        '\U00002111',
		"imath;":                           '\U00000131',
		"imof;":                            '\U000022B7',
		"imped;":                           '\U000001B5',
		"in;":                              '\U00002208',
		"incare;":                          '\U00002105',
		"infin;":                           '\U0000221E',
		"infintie;":                        '\U000029DD',
		"inodot;":                          '\U00000131',
		"int;":                             '\U0000222B',
		"intcal;":                          '\U000022BA',
		"integers;":                        '\U00002124',
		"intercal;":                        '\U000022BA',
		"intlarhk;":                        '\U00002A17',
		"intprod;":                         '\U00002A3C',
		"iocy;":                            '\U00000451',
		"iogon;":                           '\U0000012F',
		"iopf;":                            '\U0001D55A',
		"iota;":                            '\U000003B9',
		"iprod;":                           '\U00002A3C',
		"iquest;":                          '\U000000BF',
		"iscr;":                            '\U0001D4BE',
		"isin;":                            '\U00002208',
		"isinE;":                           '\U000022F9',
		"isindot;":                         '\U000022F5',
		"isins;":                           '\U000022F4',
		"isinsv;":                          '\U000022F3',
		"isinv;":                           '\U00002208',
		"it;":                              '\U00002062',
		"itilde;":                          '\U00000129',
		"iukcy;":                           '\U00000456',
		"iuml;":                            '\U000000EF',
		"jcirc;":                           '\U00000135',
		"jcy;":                             '\U00000439',
		"jfr;":                             '\U0001D527',
		"jmath;":                           '\U00000237',
		"jopf;":                            '\U0001D55B',
		"jscr;":                            '\U0001D4BF',
		"jsercy;":                          '\U00000458',
		"jukcy;":                           '\U00000454',
		"kappa;":                           '\U000003BA',
		"kappav;":                          '\U000003F0',
		"kcedil;":                          '\U00000137',
		"kcy;":                             '\U0000043A',
		"kfr;":                             '\U0001D528',
		"kgreen;":                          '\U00000138',
		"khcy;":                            '\U00000445',
		"kjcy;":                            '\U0000045C',
		"kopf;":                            '\U0001D55C',
		"kscr;":                            '\U0001D4C0',
		"lAarr;":                           '\U000021DA',
		"lArr;":                            '\U000021D0',
		"lAtail;":                          '\U0000291B',
		"lBarr;":                           '\U0000290E',
		"lE;":                              '\U00002266',
		"lEg;":                             '\U00002A8B',
		"lHar;":                            '\U00002962',
		"lacute;":                          '\U0000013A',
		"laemptyv;":                        '\U000029B4',
		"lagran;":                          '\U00002112',
		"lambda;":                          '\U000003BB',
		"lang;":                            '\U000027E8',
		"langd;":                           '\U00002991',
		"langle;":                          '\U000027E8',
		"lap;":                             '\U00002A85',
		"laquo;":                           '\U000000AB',
		"larr;":                            '\U00002190',
		"larrb;":                           '\U000021E4',
		"larrbfs;":                         '\U0000291F',
		"larrfs;":                          '\U0000291D',
		"larrhk;":                          '\U000021A9',
		"larrlp;":                          '\U000021AB',
		"larrpl;":                          '\U00002939',
		"larrsim;":                         '\U00002973',
		"larrtl;":                          '\U000021A2',
		"lat;":                             '\U00002AAB',
		"latail;":                          '\U00002919',
		"late;":                            '\U00002AAD',
		"lbarr;":                           '\U0000290C',
		"lbbrk;":                           '\U00002772',
		"lbrace;":                          '\U0000007B',
		"lbrack;":                          '\U0000005B',
		"lbrke;":                           '\U0000298B',
		"lbrksld;":                         '\U0000298F',
		"lbrkslu;":                         '\U0000298D',
		"lcaron;":                          '\U0000013E',
		"lcedil;":                          '\U0000013C',
		"lceil;":                           '\U00002308',
		"lcub;":                            '\U0000007B',
		"lcy;":                             '\U0000043B',
		"ldca;":                            '\U00002936',
		"ldquo;":                           '\U0000201C',
		"ldquor;":                          '\U0000201E',
		"ldrdhar;":                         '\U00002967',
		"ldrushar;":                        '\U0000294B',
		"ldsh;":                            '\U000021B2',
		"le;":                              '\U00002264',
		"leftarrow;":                       '\U00002190',
		"leftarrowtail;":                   '\U000021A2',
		"leftharpoondown;":                 '\U000021BD',
		"leftharpoonup;":                   '\U000021BC',
		"leftleftarrows;":                  '\U000021C7',
		"leftrightarrow;":                  '\U00002194',
		"leftrightarrows;":                 '\U000021C6',
		"leftrightharpoons;":               '\U000021CB',
		"leftrightsquigarrow;":             '\U000021AD',
		"leftthreetimes;":                  '\U000022CB',
		"leg;":                             '\U000022DA',
		"leq;":                             '\U00002264',
		"leqq;":                            '\U00002266',
		"leqslant;":                        '\U00002A7D',
		"les;":                             '\U00002A7D',
		"lescc;":                           '\U00002AA8',
		"lesdot;":                          '\U00002A7F',
		"lesdoto;":                         '\U00002A81',
		"lesdotor;":                        '\U00002A83',
		"lesges;":                          '\U00002A93',
		"lessapprox;":                      '\U00002A85',
		"lessdot;":                         '\U000022D6',
		"lesseqgtr;":                       '\U000022DA',
		"lesseqqgtr;":                      '\U00002A8B',
		"lessgtr;":                         '\U00002276',
		"lesssim;":                         '\U00002272',
		"lfisht;":                          '\U0000297C',
		"lfloor;":                          '\U0000230A',
		"lfr;":                             '\U0001D529',
		"lg;":                              '\U00002276',
		"lgE;":                             '\U00002A91',
		"lhard;":                           '\U000021BD',
		"lharu;":                           '\U000021BC',
		"lharul;":                          '\U0000296A',
		"lhblk;":                           '\U00002584',
		"ljcy;":                            '\U00000459',
		"ll;":                              '\U0000226A',
		"llarr;":                           '\U000021C7',
		"llcorner;":                        '\U0000231E',
		"llhard;":                          '\U0000296B',
		"lltri;":                           '\U000025FA',
		"lmidot;":                          '\U00000140',
		"lmoust;":                          '\U000023B0',
		"lmoustache;":                      '\U000023B0',
		"lnE;":                             '\U00002268',
		"lnap;":                            '\U00002A89',
		"lnapprox;":                        '\U00002A89',
		"lne;":                             '\U00002A87',
		"lneq;":                            '\U00002A87',
		"lneqq;":                           '\U00002268',
		"lnsim;":                           '\U000022E6',
		"loang;":                           '\U000027EC',
		"loarr;":                           '\U000021FD',
		"lobrk;":                           '\U000027E6',
		"longleftarrow;":                   '\U000027F5',
		"longleftrightarrow;":              '\U000027F7',
		"longmapsto;":                      '\U000027FC',
		"longrightarrow;":                  '\U000027F6',
		"looparrowleft;":                   '\U000021AB',
		"looparrowright;":                  '\U000021AC',
		"lopar;":                           '\U00002985',
		"lopf;":                            '\U0001D55D',
		"loplus;":                          '\U00002A2D',
		"lotimes;":                         '\U00002A34',
		"lowast;":                          '\U00002217',
		"lowbar;":                          '\U0000005F',
		"loz;":                             '\U000025CA',
		"lozenge;":                         '\U000025CA',
		"lozf;":                            '\U000029EB',
		"lpar;":                            '\U00000028',
		"lparlt;":                          '\U00002993',
		"lrarr;":                           '\U000021C6',
		"lrcorner;":                        '\U0000231F',
		"lrhar;":                           '\U000021CB',
		"lrhard;":                          '\U0000296D',
		"lrm;":                             '\U0000200E',
		"lrtri;":                           '\U000022BF',
		"lsaquo;":                          '\U00002039',
		"lscr;":                            '\U0001D4C1',
		"lsh;":                             '\U000021B0',
		"lsim;":                            '\U00002272',
		"lsime;":                           '\U00002A8D',
		"lsimg;":                           '\U00002A8F',
		"lsqb;":                            '\U0000005B',
		"lsquo;":                           '\U00002018',
		"lsquor;":                          '\U0000201A',
		"lstrok;":                          '\U00000142',
		"lt;":                              '\U0000003C',
		"ltcc;":                            '\U00002AA6',
		"ltcir;":                           '\U00002A79',
		"ltdot;":                           '\U000022D6',
		"lthree;":                          '\U000022CB',
		"ltimes;":                          '\U000022C9',
		"ltlarr;":                          '\U00002976',
		"ltquest;":                         '\U00002A7B',
		"ltrPar;":                          '\U00002996',
		"ltri;":                            '\U000025C3',
		"ltrie;":                           '\U000022B4',
		"ltrif;":                           '\U000025C2',
		"lurdshar;":                        '\U0000294A',
		"luruhar;":                         '\U00002966',
		"mDDot;":                           '\U0000223A',
		"macr;":                            '\U000000AF',
		"male;":                            '\U00002642',
		"malt;":                            '\U00002720',
		"maltese;":                         '\U00002720',
		"map;":                             '\U000021A6',
		"mapsto;":                          '\U000021A6',
		"mapstodown;":                      '\U000021A7',
		"mapstoleft;":                      '\U000021A4',
		"mapstoup;":                        '\U000021A5',
		"marker;":                          '\U000025AE',
		"mcomma;":                          '\U00002A29',
		"mcy;":                             '\U0000043C',
		"mdash;":                           '\U00002014',
		"measuredangle;":                   '\U00002221',
		"mfr;":                             '\U0001D52A',
		"mho;":                             '\U00002127',
		"micro;":                           '\U000000B5',
		"mid;":                             '\U00002223',
		"midast;":                          '\U0000002A',
		"midcir;":                          '\U00002AF0',
		"middot;":                          '\U000000B7',
		"minus;":                           '\U00002212',
		"minusb;":                          '\U0000229F',
		"minusd;":                          '\U00002238',
		"minusdu;":                         '\U00002A2A',
		"mlcp;":                            '\U00002ADB',
		"mldr;":                            '\U00002026',
		"mnplus;":                          '\U00002213',
		"models;":                          '\U000022A7',
		"mopf;":                            '\U0001D55E',
		"mp;":                              '\U00002213',
		"mscr;":                            '\U0001D4C2',
		"mstpos;":                          '\U0000223E',
		"mu;":                              '\U000003BC',
		"multimap;":                        '\U000022B8',
		"mumap;":                           '\U000022B8',
		"nLeftarrow;":                      '\U000021CD',
		"nLeftrightarrow;":                 '\U000021CE',
		"nRightarrow;":                     '\U000021CF',
		"nVDash;":                          '\U000022AF',
		"nVdash;":                          '\U000022AE',
		"nabla;":                           '\U00002207',
		"nacute;":                          '\U00000144',
		"nap;":                             '\U00002249',
		"napos;":                           '\U00000149',
		"napprox;":                         '\U00002249',
		"natur;":                           '\U0000266E',
		"natural;":                         '\U0000266E',
		"naturals;":                        '\U00002115',
		"nbsp;":                            '\U000000A0',
		"ncap;":                            '\U00002A43',
		"ncaron;":                          '\U00000148',
		"ncedil;":                          '\U00000146',
		"ncong;":                           '\U00002247',
		"ncup;":                            '\U00002A42',
		"ncy;":                             '\U0000043D',
		"ndash;":                           '\U00002013',
		"ne;":                              '\U00002260',
		"neArr;":                           '\U000021D7',
		"nearhk;":                          '\U00002924',
		"nearr;":                           '\U00002197',
		"nearrow;":                         '\U00002197',
		"nequiv;":                          '\U00002262',
		"nesear;":                          '\U00002928',
		"nexist;":                          '\U00002204',
		"nexists;":                         '\U00002204',
		"nfr;":                             '\U0001D52B',
		"nge;":                             '\U00002271',
		"ngeq;":                            '\U00002271',
		"ngsim;":                           '\U00002275',
		"ngt;":                             '\U0000226F',
		"ngtr;":                            '\U0000226F',
		"nhArr;":                           '\U000021CE',
		"nharr;":                           '\U000021AE',
		"nhpar;":                           '\U00002AF2',
		"ni;":                              '\U0000220B',
		"nis;":                             '\U000022FC',
		"nisd;":                            '\U000022FA',
		"niv;":                             '\U0000220B',
		"njcy;":                            '\U0000045A',
		"nlArr;":                           '\U000021CD',
		"nlarr;":                           '\U0000219A',
		"nldr;":                            '\U00002025',
		"nle;":                             '\U00002270',
		"nleftarrow;":                      '\U0000219A',
		"nleftrightarrow;":                 '\U000021AE',
		"nleq;":                            '\U00002270',
		"nless;":                           '\U0000226E',
		"nlsim;":                           '\U00002274',
		"nlt;":                             '\U0000226E',
		"nltri;":                           '\U000022EA',
		"nltrie;":                          '\U000022EC',
		"nmid;":                            '\U00002224',
		"nopf;":                            '\U0001D55F',
		"not;":                             '\U000000AC',
		"notin;":                           '\U00002209',
		"notinva;":                         '\U00002209',
		"notinvb;":                         '\U000022F7',
		"notinvc;":                         '\U000022F6',
		"notni;":                           '\U0000220C',
		"notniva;":                         '\U0000220C',
		"notnivb;":                         '\U000022FE',
		"notnivc;":                         '\U000022FD',
		"npar;":                            '\U00002226',
		"nparallel;":                       '\U00002226',
		"npolint;":                         '\U00002A14',
		"npr;":                             '\U00002280',
		"nprcue;":                          '\U000022E0',
		"nprec;":                           '\U00002280',
		"nrArr;":                           '\U000021CF',
		"nrarr;":                           '\U0000219B',
		"nrightarrow;":                     '\U0000219B',
		"nrtri;":                           '\U000022EB',
		"nrtrie;":                          '\U000022ED',
		"nsc;":                             '\U00002281',
		"nsccue;":                          '\U000022E1',
		"nscr;":                            '\U0001D4C3',
		"nshortmid;":                       '\U00002224',
		"nshortparallel;":                  '\U00002226',
		"nsim;":                            '\U00002241',
		"nsime;":                           '\U00002244',
		"nsimeq;":                          '\U00002244',
		"nsmid;":                           '\U00002224',
		"nspar;":                           '\U00002226',
		"nsqsube;":                         '\U000022E2',
		"nsqsupe;":                         '\U000022E3',
		"nsub;":                            '\U00002284',
		"nsube;":                           '\U00002288',
		"nsubseteq;":                       '\U00002288',
		"nsucc;":                           '\U00002281',
		"nsup;":                            '\U00002285',
		"nsupe;":                           '\U00002289',
		"nsupseteq;":                       '\U00002289',
		"ntgl;":                            '\U00002279',
		"ntilde;":                          '\U000000F1',
		"ntlg;":                            '\U00002278',
		"ntriangleleft;":                   '\U000022EA',
		"ntrianglelefteq;":                 '\U000022EC',
		"ntriangleright;":                  '\U000022EB',
		"ntrianglerighteq;":                '\U000022ED',
		"nu;":                              '\U000003BD',
		"num;":                             '\U00000023',
		"numero;":                          '\U00002116',
		"numsp;":                           '\U00002007',
		"nvDash;":                          '\U000022AD',
		"nvHarr;":                          '\U00002904',
		"nvdash;":                          '\U000022AC',
		"nvinfin;":                         '\U000029DE',
		"nvlArr;":                          '\U00002902',
		"nvrArr;":                          '\U00002903',
		"nwArr;":                           '\U000021D6',
		"nwarhk;":                          '\U00002923',
		"nwarr;":                           '\U00002196',
		"nwarrow;":                         '\U00002196',
		"nwnear;":                          '\U00002927',
		"oS;":                              '\U000024C8',
		"oacute;":                          '\U000000F3',
		"oast;":                            '\U0000229B',
		"ocir;":                            '\U0000229A',
		"ocirc;":                           '\U000000F4',
		"ocy;":                             '\U0000043E',
		"odash;":                           '\U0000229D',
		"odblac;":                          '\U00000151',
		"odiv;":                            '\U00002A38',
		"odot;":                            '\U00002299',
		"odsold;":                          '\U000029BC',
		"oelig;":                           '\U00000153',
		"ofcir;":                           '\U000029BF',
		"ofr;":                             '\U0001D52C',
		"ogon;":                            '\U000002DB',
		"ograve;":                          '\U000000F2',
		"ogt;":                             '\U000029C1',
		"ohbar;":                           '\U000029B5',
		"ohm;":                             '\U000003A9',
		"oint;":                            '\U0000222E',
		"olarr;":                           '\U000021BA',
		"olcir;":                           '\U000029BE',
		"olcross;":                         '\U000029BB',
		"oline;":                           '\U0000203E',
		"olt;":                             '\U000029C0',
		"omacr;":                           '\U0000014D',
		"omega;":                           '\U000003C9',
		"omicron;":                         '\U000003BF',
		"omid;":                            '\U000029B6',
		"ominus;":                          '\U00002296',
		"oopf;":                            '\U0001D560',
		"opar;":                            '\U000029B7',
		"operp;":                           '\U000029B9',
		"oplus;":                           '\U00002295',
		"or;":                              '\U00002228',
		"orarr;":                           '\U000021BB',
		"ord;":                             '\U00002A5D',
		"order;":                           '\U00002134',
		"orderof;":                         '\U00002134',
		"ordf;":                            '\U000000AA',
		"ordm;":                            '\U000000BA',
		"origof;":                          '\U000022B6',
		"oror;":                            '\U00002A56',
		"orslope;":                         '\U00002A57',
		"orv;":                             '\U00002A5B',
		"oscr;":                            '\U00002134',
		"oslash;":                          '\U000000F8',
		"osol;":                            '\U00002298',
		"otilde;":                          '\U000000F5',
		"otimes;":                          '\U00002297',
		"otimesas;":                        '\U00002A36',
		"ouml;":                            '\U000000F6',
		"ovbar;":                           '\U0000233D',
		"par;":                             '\U00002225',
		"para;":                            '\U000000B6',
		"parallel;":                        '\U00002225',
		"parsim;":                          '\U00002AF3',
		"parsl;":                           '\U00002AFD',
		"part;":                            '\U00002202',
		"pcy;":                             '\U0000043F',
		"percnt;":                          '\U00000025',
		"period;":                          '\U0000002E',
		"permil;":                          '\U00002030',
		"perp;":                            '\U000022A5',
		"pertenk;":                         '\U00002031',
		"pfr;":                             '\U0001D52D',
		"phi;":                             '\U000003C6',
		"phiv;":                            '\U000003D5',
		"phmmat;":                          '\U00002133',
		"phone;":                           '\U0000260E',
		"pi;":                              '\U000003C0',
		"pitchfork;":                       '\U000022D4',
		"piv;":                             '\U000003D6',
		"planck;":                          '\U0000210F',
		"planckh;":                         '\U0000210E',
		"plankv;":                          '\U0000210F',
		"plus;":                            '\U0000002B',
		"plusacir;":                        '\U00002A23',
		"plusb;":                           '\U0000229E',
		"pluscir;":                         '\U00002A22',
		"plusdo;":                          '\U00002214',
		"plusdu;":                          '\U00002A25',
		"pluse;":                           '\U00002A72',
		"plusmn;":                          '\U000000B1',
		"plussim;":                         '\U00002A26',
		"plustwo;":                         '\U00002A27',
		"pm;":                              '\U000000B1',
		"pointint;":                        '\U00002A15',
		"popf;":                            '\U0001D561',
		"pound;":                           '\U000000A3',
		"pr;":                              '\U0000227A',
		"prE;":                             '\U00002AB3',
		"prap;":                            '\U00002AB7',
		"prcue;":                           '\U0000227C',
		"pre;":                             '\U00002AAF',
		"prec;":                            '\U0000227A',
		"precapprox;":                      '\U00002AB7',
		"preccurlyeq;":                     '\U0000227C',
		"preceq;":                          '\U00002AAF',
		"precnapprox;":                     '\U00002AB9',
		"precneqq;":                        '\U00002AB5',
		"precnsim;":                        '\U000022E8',
		"precsim;":                         '\U0000227E',
		"prime;":                           '\U00002032',
		"primes;":                          '\U00002119',
		"prnE;":                            '\U00002AB5',
		"prnap;":                           '\U00002AB9',
		"prnsim;":                          '\U000022E8',
		"prod;":                            '\U0000220F',
		"profalar;":                        '\U0000232E',
		"profline;":                        '\U00002312',
		"profsurf;":                        '\U00002313',
		"prop;":                            '\U0000221D',
		"propto;":                          '\U0000221D',
		"prsim;":                           '\U0000227E',
		"prurel;":                          '\U000022B0',
		"pscr;":                            '\U0001D4C5',
		"psi;":                             '\U000003C8',
		"puncsp;":                          '\U00002008',
		"qfr;":                             '\U0001D52E',
		"qint;":                            '\U00002A0C',
		"qopf;":                            '\U0001D562',
		"qprime;":                          '\U00002057',
		"qscr;":                            '\U0001D4C6',
		"quaternions;":                     '\U0000210D',
		"quatint;":                         '\U00002A16',
		"quest;":                           '\U0000003F',
		"questeq;":                         '\U0000225F',
		"quot;":                            '\U00000022',
		"rAarr;":                           '\U000021DB',
		"rArr;":                            '\U000021D2',
		"rAtail;":                          '\U0000291C',
		"rBarr;":                           '\U0000290F',
		"rHar;":                            '\U00002964',
		"racute;":                          '\U00000155',
		"radic;":                           '\U0000221A',
		"raemptyv;":                        '\U000029B3',
		"rang;":                            '\U000027E9',
		"rangd;":                           '\U00002992',
		"range;":                           '\U000029A5',
		"rangle;":                          '\U000027E9',
		"raquo;":                           '\U000000BB',
		"rarr;":                            '\U00002192',
		"rarrap;":                          '\U00002975',
		"rarrb;":                           '\U000021E5',
		"rarrbfs;":                         '\U00002920',
		"rarrc;":                           '\U00002933',
		"rarrfs;":                          '\U0000291E',
		"rarrhk;":                          '\U000021AA',
		"rarrlp;":                          '\U000021AC',
		"rarrpl;":                          '\U00002945',
		"rarrsim;":                         '\U00002974',
		"rarrtl;":                          '\U000021A3',
		"rarrw;":                           '\U0000219D',
		"ratail;":                          '\U0000291A',
		"ratio;":                           '\U00002236',
		"rationals;":                       '\U0000211A',
		"rbarr;":                           '\U0000290D',
		"rbbrk;":                           '\U00002773',
		"rbrace;":                          '\U0000007D',
		"rbrack;":                          '\U0000005D',
		"rbrke;":                           '\U0000298C',
		"rbrksld;":                         '\U0000298E',
		"rbrkslu;":                         '\U00002990',
		"rcaron;":                          '\U00000159',
		"rcedil;":                          '\U00000157',
		"rceil;":                           '\U00002309',
		"rcub;":                            '\U0000007D',
		"rcy;":                             '\U00000440',
		"rdca;":                            '\U00002937',
		"rdldhar;":                         '\U00002969',
		"rdquo;":                           '\U0000201D',
		"rdquor;":                          '\U0000201D',
		"rdsh;":                            '\U000021B3',
		"real;":                            '\U0000211C',
		"realine;":                         '\U0000211B',
		"realpart;":                        '\U0000211C',
		"reals;":                           '\U0000211D',
		"rect;":                            '\U000025AD',
		"reg;":                             '\U000000AE',
		"rfisht;":                          '\U0000297D',
		"rfloor;":                          '\U0000230B',
		"rfr;":                             '\U0001D52F',
		"rhard;":                           '\U000021C1',
		"rharu;":                           '\U000021C0',
		"rharul;":                          '\U0000296C',
		"rho;":                             '\U000003C1',
		"rhov;":                            '\U000003F1',
		"rightarrow;":                      '\U00002192',
		"rightarrowtail;":                  '\U000021A3',
		"rightharpoondown;":                '\U000021C1',
		"rightharpoonup;":                  '\U000021C0',
		"rightleftarrows;":                 '\U000021C4',
		"rightleftharpoons;":               '\U000021CC',
		"rightrightarrows;":                '\U000021C9',
		"rightsquigarrow;":                 '\U0000219D',
		"rightthreetimes;":                 '\U000022CC',
		"ring;":                            '\U000002DA',
		"risingdotseq;":                    '\U00002253',
		"rlarr;":                           '\U000021C4',
		"rlhar;":                           '\U000021CC',
		"rlm;":                             '\U0000200F',
		"rmoust;":                          '\U000023B1',
		"rmoustache;":                      '\U000023B1',
		"rnmid;":                           '\U00002AEE',
		"roang;":                           '\U000027ED',
		"roarr;":                           '\U000021FE',
		"robrk;":                           '\U000027E7',
		"ropar;":                           '\U00002986',
		"ropf;":                            '\U0001D563',
		"roplus;":                          '\U00002A2E',
		"rotimes;":                         '\U00002A35',
		"rpar;":                            '\U00000029',
		"rpargt;":                          '\U00002994',
		"rppolint;":                        '\U00002A12',
		"rrarr;":                           '\U000021C9',
		"rsaquo;":                          '\U0000203A',
		"rscr;":                            '\U0001D4C7',
		"rsh;":                             '\U000021B1',
		"rsqb;":                            '\U0000005D',
		"rsquo;":                           '\U00002019',
		"rsquor;":                          '\U00002019',
		"rthree;":                          '\U000022CC',
		"rtimes;":                          '\U000022CA',
		"rtri;":                            '\U000025B9',
		"rtrie;":                           '\U000022B5',
		"rtrif;":                           '\U000025B8',
		"rtriltri;":                        '\U000029CE',
		"ruluhar;":                         '\U00002968',
		"rx;":                              '\U0000211E',
		"sacute;":                          '\U0000015B',
		"sbquo;":                           '\U0000201A',
		"sc;":                              '\U0000227B',
		"scE;":                             '\U00002AB4',
		"scap;":                            '\U00002AB8',
		"scaron;":                          '\U00000161',
		"sccue;":                           '\U0000227D',
		"sce;":                             '\U00002AB0',
		"scedil;":                          '\U0000015F',
		"scirc;":                           '\U0000015D',
		"scnE;":                            '\U00002AB6',
		"scnap;":                           '\U00002ABA',
		"scnsim;":                          '\U000022E9',
		"scpolint;":                        '\U00002A13',
		"scsim;":                           '\U0000227F',
		"scy;":                             '\U00000441',
		"sdot;":                            '\U000022C5',
		"sdotb;":                           '\U000022A1',
		"sdote;":                           '\U00002A66',
		"seArr;":                           '\U000021D8',
		"searhk;":                          '\U00002925',
		"searr;":                           '\U00002198',
		"searrow;":                         '\U00002198',
		"sect;":                            '\U000000A7',
		"semi;":                            '\U0000003B',
		"seswar;":                          '\U00002929',
		"setminus;":                        '\U00002216',
		"setmn;":                           '\U00002216',
		"sext;":                            '\U00002736',
		"sfr;":                             '\U0001D530',
		"sfrown;":                          '\U00002322',
		"sharp;":                           '\U0000266F',
		"shchcy;":                          '\U00000449',
		"shcy;":                            '\U00000448',
		"shortmid;":                        '\U00002223',
		"shortparallel;":                   '\U00002225',
		"shy;":                             '\U000000AD',
		"sigma;":                           '\U000003C3',
		"sigmaf;":                          '\U000003C2',
		"sigmav;":                          '\U000003C2',
		"sim;":                             '\U0000223C',
		"simdot;":                          '\U00002A6A',
		"sime;":                            '\U00002243',
		"simeq;":                           '\U00002243',
		"simg;":                            '\U00002A9E',
		"simgE;":                           '\U00002AA0',
		"siml;":                            '\U00002A9D',
		"simlE;":                           '\U00002A9F',
		"simne;":                           '\U00002246',
		"simplus;":                         '\U00002A24',
		"simrarr;":                         '\U00002972',
		"slarr;":                           '\U00002190',
		"smallsetminus;":                   '\U00002216',
		"smashp;":                          '\U00002A33',
		"smeparsl;":                        '\U000029E4',
		"smid;":                            '\U00002223',
		"smile;":                           '\U00002323',
		"smt;":                             '\U00002AAA',
		"smte;":                            '\U00002AAC',
		"softcy;":                          '\U0000044C',
		"sol;":                             '\U0000002F',
		"solb;":                            '\U000029C4',
		"solbar;":                          '\U0000233F',
		"sopf;":                            '\U0001D564',
		"spades;":                          '\U00002660',
		"spadesuit;":                       '\U00002660',
		"spar;":                            '\U00002225',
		"sqcap;":                           '\U00002293',
		"sqcup;":                           '\U00002294',
		"sqsub;":                           '\U0000228F',
		"sqsube;":                          '\U00002291',
		"sqsubset;":                        '\U0000228F',
		"sqsubseteq;":                      '\U00002291',
		"sqsup;":                           '\U00002290',
		"sqsupe;":                          '\U00002292',
		"sqsupset;":                        '\U00002290',
		"sqsupseteq;":                      '\U00002292',
		"squ;":                             '\U000025A1',
		"square;":                          '\U000025A1',
		"squarf;":                          '\U000025AA',
		"squf;":                            '\U000025AA',
		"srarr;":                           '\U00002192',
		"sscr;":                            '\U0001D4C8',
		"ssetmn;":                          '\U00002216',
		"ssmile;":                          '\U00002323',
		"sstarf;":                          '\U000022C6',
		"star;":                            '\U00002606',
		"starf;":                           '\U00002605',
		"straightepsilon;":                 '\U000003F5',
		"straightphi;":                     '\U000003D5',
		"strns;":                           '\U000000AF',
		"sub;":                             '\U00002282',
		"subE;":                            '\U00002AC5',
		"subdot;":                          '\U00002ABD',
		"sube;":                            '\U00002286',
		"subedot;":                         '\U00002AC3',
		"submult;":                         '\U00002AC1',
		"subnE;":                           '\U00002ACB',
		"subne;":                           '\U0000228A',
		"subplus;":                         '\U00002ABF',
		"subrarr;":                         '\U00002979',
		"subset;":                          '\U00002282',
		"subseteq;":                        '\U00002286',
		"subseteqq;":                       '\U00002AC5',
		"subsetneq;":                       '\U0000228A',
		"subsetneqq;":                      '\U00002ACB',
		"subsim;":                          '\U00002AC7',
		"subsub;":                          '\U00002AD5',
		"subsup;":                          '\U00002AD3',
		"succ;":                            '\U0000227B',
		"succapprox;":                      '\U00002AB8',
		"succcurlyeq;":                     '\U0000227D',
		"succeq;":                          '\U00002AB0',
		"succnapprox;":                     '\U00002ABA',
		"succneqq;":                        '\U00002AB6',
		"succnsim;":                        '\U000022E9',
		"succsim;":                         '\U0000227F',
		"sum;":                             '\U00002211',
		"sung;":                            '\U0000266A',
		"sup;":                             '\U00002283',
		"sup1;":                            '\U000000B9',
		"sup2;":                            '\U000000B2',
		"sup3;":                            '\U000000B3',
		"supE;":                            '\U00002AC6',
		"supdot;":                          '\U00002ABE',
		"supdsub;":                         '\U00002AD8',
		"supe;":                            '\U00002287',
		"supedot;":                         '\U00002AC4',
		"suphsol;":                         '\U000027C9',
		"suphsub;":                         '\U00002AD7',
		"suplarr;":                         '\U0000297B',
		"supmult;":                         '\U00002AC2',
		"supnE;":                           '\U00002ACC',
		"supne;":                           '\U0000228B',
		"supplus;":                         '\U00002AC0',
		"supset;":                          '\U00002283',
		"supseteq;":                        '\U00002287',
		"supseteqq;":                       '\U00002AC6',
		"supsetneq;":                       '\U0000228B',
		"supsetneqq;":                      '\U00002ACC',
		"supsim;":                          '\U00002AC8',
		"supsub;":                          '\U00002AD4',
		"supsup;":                          '\U00002AD6',
		"swArr;":                           '\U000021D9',
		"swarhk;":                          '\U00002926',
		"swarr;":                           '\U00002199',
		"swarrow;":                         '\U00002199',
		"swnwar;":                          '\U0000292A',
		"szlig;":                           '\U000000DF',
		"target;":                          '\U00002316',
		"tau;":                             '\U000003C4',
		"tbrk;":                            '\U000023B4',
		"tcaron;":                          '\U00000165',
		"tcedil;":                          '\U00000163',
		"tcy;":                             '\U00000442',
		"tdot;":                            '\U000020DB',
		"telrec;":                          '\U00002315',
		"tfr;":                             '\U0001D531',
		"there4;":                          '\U00002234',
		"therefore;":                       '\U00002234',
		"theta;":                           '\U000003B8',
		"thetasym;":                        '\U000003D1',
		"thetav;":                          '\U000003D1',
		"thickapprox;":                     '\U00002248',
		"thicksim;":                        '\U0000223C',
		"thinsp;":                          '\U00002009',
		"thkap;":                           '\U00002248',
		"thksim;":                          '\U0000223C',
		"thorn;":                           '\U000000FE',
		"tilde;":                           '\U000002DC',
		"times;":                           '\U000000D7',
		"timesb;":                          '\U000022A0',
		"timesbar;":                        '\U00002A31',
		"timesd;":                          '\U00002A30',
		"tint;":                            '\U0000222D',
		"toea;":                            '\U00002928',
		"top;":                             '\U000022A4',
		"topbot;":                          '\U00002336',
		"topcir;":                          '\U00002AF1',
		"topf;":                            '\U0001D565',
		"topfork;":                         '\U00002ADA',
		"tosa;":                            '\U00002929',
		"tprime;":                          '\U00002034',
		"trade;":                           '\U00002122',
		"triangle;":                        '\U000025B5',
		"triangledown;":                    '\U000025BF',
		"triangleleft;":                    '\U000025C3',
		"trianglelefteq;":                  '\U000022B4',
		"triangleq;":                       '\U0000225C',
		"triangleright;":                   '\U000025B9',
		"trianglerighteq;":                 '\U000022B5',
		"tridot;":                          '\U000025EC',
		"trie;":                            '\U0000225C',
		"triminus;":                        '\U00002A3A',
		"triplus;":                         '\U00002A39',
		"trisb;":                           '\U000029CD',
		"tritime;":                         '\U00002A3B',
		"trpezium;":                        '\U000023E2',
		"tscr;":                            '\U0001D4C9',
		"tscy;":                            '\U00000446',
		"tshcy;":                           '\U0000045B',
		"tstrok;":                          '\U00000167',
		"twixt;":                           '\U0000226C',
		"twoheadleftarrow;":                '\U0000219E',
		"twoheadrightarrow;":               '\U000021A0',
		"uArr;":                            '\U000021D1',
		"uHar;":                            '\U00002963',
		"uacute;":                          '\U000000FA',
		"uarr;":                            '\U00002191',
		"ubrcy;":                           '\U0000045E',
		"ubreve;":                          '\U0000016D',
		"ucirc;":                           '\U000000FB',
		"ucy;":                             '\U00000443',
		"udarr;":                           '\U000021C5',
		"udblac;":                          '\U00000171',
		"udhar;":                           '\U0000296E',
		"ufisht;":                          '\U0000297E',
		"ufr;":                             '\U0001D532',
		"ugrave;":                          '\U000000F9',
		"uharl;":                           '\U000021BF',
		"uharr;":                           '\U000021BE',
		"uhblk;":                           '\U00002580',
		"ulcorn;":                          '\U0000231C',
		"ulcorner;":                        '\U0000231C',
		"ulcrop;":                          '\U0000230F',
		"ultri;":                           '\U000025F8',
		"umacr;":                           '\U0000016B',
		"uml;":                             '\U000000A8',
		"uogon;":                           '\U00000173',
		"uopf;":                            '\U0001D566',
		"uparrow;":                         '\U00002191',
		"updownarrow;":                     '\U00002195',
		"upharpoonleft;":                   '\U000021BF',
		"upharpoonright;":                  '\U000021BE',
		"uplus;":                           '\U0000228E',
		"upsi;":                            '\U000003C5',
		"upsih;":                           '\U000003D2',
		"upsilon;":                         '\U000003C5',
		"upuparrows;":                      '\U000021C8',
		"urcorn;":                          '\U0000231D',
		"urcorner;":                        '\U0000231D',
		"urcrop;":                          '\U0000230E',
		"uring;":                           '\U0000016F',
		"urtri;":                           '\U000025F9',
		"uscr;":                            '\U0001D4CA',
		"utdot;":                           '\U000022F0',
		"utilde;":                          '\U00000169',
		"utri;":                            '\U000025B5',
		"utrif;":                           '\U000025B4',
		"uuarr;":                           '\U000021C8',
		"uuml;":                            '\U000000FC',
		"uwangle;":                         '\U000029A7',
		"vArr;":                            '\U000021D5',
		"vBar;":                            '\U00002AE8',
		"vBarv;":                           '\U00002AE9',
		"vDash;":                           '\U000022A8',
		"vangrt;":                          '\U0000299C',
		"varepsilon;":                      '\U000003F5',
		"varkappa;":                        '\U000003F0',
		"varnothing;":                      '\U00002205',
		"varphi;":                          '\U000003D5',
		"varpi;":                           '\U000003D6',
		"varpropto;":                       '\U0000221D',
		"varr;":                            '\U00002195',
		"varrho;":                          '\U000003F1',
		"varsigma;":                        '\U000003C2',
		"vartheta;":                        '\U000003D1',
		"vartriangleleft;":                 '\U000022B2',
		"vartriangleright;":                '\U000022B3',
		"vcy;":                             '\U00000432',
		"vdash;":                           '\U000022A2',
		"vee;":                             '\U00002228',
		"veebar;":                          '\U000022BB',
		"veeeq;":                           '\U0000225A',
		"vellip;":                          '\U000022EE',
		"verbar;":                          '\U0000007C',
		"vert;":                            '\U0000007C',
		"vfr;":                             '\U0001D533',
		"vltri;":                           '\U000022B2',
		"vopf;":                            '\U0001D567',
		"vprop;":                           '\U0000221D',
		"vrtri;":                           '\U000022B3',
		"vscr;":                            '\U0001D4CB',
		"vzigzag;":                         '\U0000299A',
		"wcirc;":                           '\U00000175',
		"wedbar;":                          '\U00002A5F',
		"wedge;":                           '\U00002227',
		"wedgeq;":                          '\U00002259',
		"weierp;":                          '\U00002118',
		"wfr;":                             '\U0001D534',
		"wopf;":                            '\U0001D568',
		"wp;":                              '\U00002118',
		"wr;":                              '\U00002240',
		"wreath;":                          '\U00002240',
		"wscr;":                            '\U0001D4CC',
		"xcap;":                            '\U000022C2',
		"xcirc;":                           '\U000025EF',
		"xcup;":                            '\U000022C3',
		"xdtri;":                           '\U000025BD',
		"xfr;":                             '\U0001D535',
		"xhArr;":                           '\U000027FA',
		"xharr;":                           '\U000027F7',
		"xi;":                              '\U000003BE',
		"xlArr;":                           '\U000027F8',
		"xlarr;":                           '\U000027F5',
		"xmap;":                            '\U000027FC',
		"xnis;":                            '\U000022FB',
		"xodot;":                           '\U00002A00',
		"xopf;":                            '\U0001D569',
		"xoplus;":                          '\U00002A01',
		"xotime;":                          '\U00002A02',
		"xrArr;":                           '\U000027F9',
		"xrarr;":                           '\U000027F6',
		"xscr;":                            '\U0001D4CD',
		"xsqcup;":                          '\U00002A06',
		"xuplus;":                          '\U00002A04',
		"xutri;":                           '\U000025B3',
		"xvee;":                            '\U000022C1',
		"xwedge;":                          '\U000022C0',
		"yacute;":                          '\U000000FD',
		"yacy;":                            '\U0000044F',
		"ycirc;":                           '\U00000177',
		"ycy;":                             '\U0000044B',
		"yen;":                             '\U000000A5',
		"yfr;":                             '\U0001D536',
		"yicy;":                            '\U00000457',
		"yopf;":                            '\U0001D56A',
		"yscr;":                            '\U0001D4CE',
		"yucy;":                            '\U0000044E',
		"yuml;":                            '\U000000FF',
		"zacute;":                          '\U0000017A',
		"zcaron;":                          '\U0000017E',
		"zcy;":                             '\U00000437',
		"zdot;":                            '\U0000017C',
		"zeetrf;":                          '\U00002128',
		"zeta;":                            '\U000003B6',
		"zfr;":                             '\U0001D537',
		"zhcy;":                            '\U00000436',
		"zigrarr;":                         '\U000021DD',
		"zopf;":                            '\U0001D56B',
		"zscr;":                            '\U0001D4CF',
		"zwj;":                             '\U0000200D',
		"zwnj;":                            '\U0000200C',
		"AElig":                            '\U000000C6',
		"AMP":                              '\U00000026',
		"Aacute":                           '\U000000C1',
		"Acirc":                            '\U000000C2',
		"Agrave":                           '\U000000C0',
		"Aring":                            '\U000000C5',
		"Atilde":                           '\U000000C3',
		"Auml":                             '\U000000C4',
		"COPY":                             '\U000000A9',
		"Ccedil":                           '\U000000C7',
		"ETH":                              '\U000000D0',
		"Eacute":                           '\U000000C9',
		"Ecirc":                            '\U000000CA',
		"Egrave":                           '\U000000C8',
		"Euml":                             '\U000000CB',
		"GT":                               '\U0000003E',
		"Iacute":                           '\U000000CD',
		"Icirc":                            '\U000000CE',
		"Igrave":                           '\U000000CC',
		"Iuml":                             '\U000000CF',
		"LT":                               '\U0000003C',
		"Ntilde":                           '\U000000D1',
		"Oacute":                           '\U000000D3',
		"Ocirc":                            '\U000000D4',
		"Ograve":                           '\U000000D2',
		"Oslash":                           '\U000000D8',
		"Otilde":                           '\U000000D5',
		"Ouml":                             '\U000000D6',
		"QUOT":                             '\U00000022',
		"REG":                              '\U000000AE',
		"THORN":                            '\U000000DE',
		"Uacute":                           '\U000000DA',
		"Ucirc":                            '\U000000DB',
		"Ugrave":                           '\U000000D9',
		"Uuml":                             '\U000000DC',
		"Yacute":                           '\U000000DD',
		"aacute":                           '\U000000E1',
		"acirc":                            '\U000000E2',
		"acute":                            '\U000000B4',
		"aelig":                            '\U000000E6',
		"agrave":                           '\U000000E0',
		"amp":                              '\U00000026',
		"aring":                            '\U000000E5',
		"atilde":                           '\U000000E3',
		"auml":                             '\U000000E4',
		"brvbar":                           '\U000000A6',
		"ccedil":                           '\U000000E7',
		"cedil":                            '\U000000B8',
		"cent":                             '\U000000A2',
		"copy":                             '\U000000A9',
		"curren":                           '\U000000A4',
		"deg":                              '\U000000B0',
		"divide":                           '\U000000F7',
		"eacute":                           '\U000000E9',
		"ecirc":                            '\U000000EA',
		"egrave":                           '\U000000E8',
		"eth":                              '\U000000F0',
		"euml":                             '\U000000EB',
		"frac12":                           '\U000000BD',
		"frac14":                           '\U000000BC',
		"frac34":                           '\U000000BE',
		"gt":                               '\U0000003E',
		"iacute":                           '\U000000ED',
		"icirc":                            '\U000000EE',
		"iexcl":                            '\U000000A1',
		"igrave":                           '\U000000EC',
		"iquest":                           '\U000000BF',
		"iuml":                             '\U000000EF',
		"laquo":                            '\U000000AB',
		"lt":                               '\U0000003C',
		"macr":                             '\U000000AF',
		"micro":                            '\U000000B5',
		"middot":                           '\U000000B7',
		"nbsp":                             '\U000000A0',
		"not":                              '\U000000AC',
		"ntilde":                           '\U000000F1',
		"oacute":                           '\U000000F3',
		"ocirc":                            '\U000000F4',
		"ograve":                           '\U000000F2',
		"ordf":                             '\U000000AA',
		"ordm":                             '\U000000BA',
		"oslash":                           '\U000000F8',
		"otilde":                           '\U000000F5',
		"ouml":                             '\U000000F6',
		"para":                             '\U000000B6',
		"plusmn":                           '\U000000B1',
		"pound":                            '\U000000A3',
		"quot":                             '\U00000022',
		"raquo":                            '\U000000BB',
		"reg":                              '\U000000AE',
		"sect":                             '\U000000A7',
		"shy":                              '\U000000AD',
		"sup1":                             '\U000000B9',
		"sup2":                             '\U000000B2',
		"sup3":                             '\U000000B3',
		"szlig":                            '\U000000DF',
		"thorn":                            '\U000000FE',
		"times":                            '\U000000D7',
		"uacute":                           '\U000000FA',
		"ucirc":                            '\U000000FB',
		"ugrave":                           '\U000000F9',
		"uml":                              '\U000000A8',
		"uuml":                             '\U000000FC',
		"yacute":                           '\U000000FD',
		"yen":                              '\U000000A5',
		"yuml":                             '\U000000FF',
	}

	entity2 = map[string][2]rune{
		// TODO(nigeltao): Handle replacements that are wider than their names.
		// "nLt;":                     {'\u226A', '\u20D2'},
		// "nGt;":                     {'\u226B', '\u20D2'},
		"NotEqualTilde;":           {'\u2242', '\u0338'},
		"NotGreaterFullEqual;":     {'\u2267', '\u0338'},
		"NotGreaterGreater;":       {'\u226B', '\u0338'},
		"NotGreaterSlantEqual;":    {'\u2A7E', '\u0338'},
		"NotHumpDownHump;":         {'\u224E', '\u0338'},
		"NotHumpEqual;":            {'\u224F', '\u0338'},
		"NotLeftTriangleBar;":      {'\u29CF', '\u0338'},
		"NotLessLess;":             {'\u226A', '\u0338'},
		"NotLessSlantEqual;":       {'\u2A7D', '\u0338'},
		"NotNestedGreaterGreater;": {'\u2AA2', '\u0338'},
		"NotNestedLessLess;":       {'\u2AA1', '\u0338'},
		"NotPrecedesEqual;":        {'\u2AAF', '\u0338'},
		"NotRightTriangleBar;":     {'\u29D0', '\u0338'},
		"NotSquareSubset;":         {'\u228F', '\u0338'},
		"NotSquareSuperset;":       {'\u2290', '\u0338'},
		"NotSubset;":               {'\u2282', '\u20D2'},
		"NotSucceedsEqual;":        {'\u2AB0', '\u0338'},
		"NotSucceedsTilde;":        {'\u227F', '\u0338'},
		"NotSuperset;":             {'\u2283', '\u20D2'},
		"ThickSpace;":              {'\u205F', '\u200A'},
		"acE;":                     {'\u223E', '\u0333'},
		"bne;":                     {'\u003D', '\u20E5'},
		"bnequiv;":                 {'\u2261', '\u20E5'},
		"caps;":                    {'\u2229', '\uFE00'},
		"cups;":                    {'\u222A', '\uFE00'},
		"fjlig;":                   {'\u0066', '\u006A'},
		"gesl;":                    {'\u22DB', '\uFE00'},
		"gvertneqq;":               {'\u2269', '\uFE00'},
		"gvnE;":                    {'\u2269', '\uFE00'},
		"lates;":                   {'\u2AAD', '\uFE00'},
		"lesg;":                    {'\u22DA', '\uFE00'},
		"lvertneqq;":               {'\u2268', '\uFE00'},
		"lvnE;":                    {'\u2268', '\uFE00'},
		"nGg;":                     {'\u22D9', '\u0338'},
		"nGtv;":                    {'\u226B', '\u0338'},
		"nLl;":                     {'\u22D8', '\u0338'},
		"nLtv;":                    {'\u226A', '\u0338'},
		"nang;":                    {'\u2220', '\u20D2'},
		"napE;":                    {'\u2A70', '\u0338'},
		"napid;":                   {'\u224B', '\u0338'},
		"nbump;":                   {'\u224E', '\u0338'},
		"nbumpe;":                  {'\u224F', '\u0338'},
		"ncongdot;":                {'\u2A6D', '\u0338'},
		"nedot;":                   {'\u2250', '\u0338'},
		"nesim;":                   {'\u2242', '\u0338'},
		"ngE;":                     {'\u2267', '\u0338'},
		"ngeqq;":                   {'\u2267', '\u0338'},
		"ngeqslant;":               {'\u2A7E', '\u0338'},
		"nges;":                    {'\u2A7E', '\u0338'},
		"nlE;":                     {'\u2266', '\u0338'},
		"nleqq;":                   {'\u2266', '\u0338'},
		"nleqslant;":               {'\u2A7D', '\u0338'},
		"nles;":                    {'\u2A7D', '\u0338'},
		"notinE;":                  {'\u22F9', '\u0338'},
		"notindot;":                {'\u22F5', '\u0338'},
		"nparsl;":                  {'\u2AFD', '\u20E5'},
		"npart;":                   {'\u2202', '\u0338'},
		"npre;":                    {'\u2AAF', '\u0338'},
		"npreceq;":                 {'\u2AAF', '\u0338'},
		"nrarrc;":                  {'\u2933', '\u0338'},
		"nrarrw;":                  {'\u219D', '\u0338'},
		"nsce;":                    {'\u2AB0', '\u0338'},
		"nsubE;":                   {'\u2AC5', '\u0338'},
		"nsubset;":                 {'\u2282', '\u20D2'},
		"nsubseteqq;":              {'\u2AC5', '\u0338'},
		"nsucceq;":                 {'\u2AB0', '\u0338'},
		"nsupE;":                   {'\u2AC6', '\u0338'},
		"nsupset;":                 {'\u2283', '\u20D2'},
		"nsupseteqq;":              {'\u2AC6', '\u0338'},
		"nvap;":                    {'\u224D', '\u20D2'},
		"nvge;":                    {'\u2265', '\u20D2'},
		"nvgt;":                    {'\u003E', '\u20D2'},
		"nvle;":                    {'\u2264', '\u20D2'},
		"nvlt;":                    {'\u003C', '\u20D2'},
		"nvltrie;":                 {'\u22B4', '\u20D2'},
		"nvrtrie;":                 {'\u22B5', '\u20D2'},
		"nvsim;":                   {'\u223C', '\u20D2'},
		"race;":                    {'\u223D', '\u0331'},
		"smtes;":                   {'\u2AAC', '\uFE00'},
		"sqcaps;":                  {'\u2293', '\uFE00'},
		"sqcups;":                  {'\u2294', '\uFE00'},
		"varsubsetneq;":            {'\u228A', '\uFE00'},
		"varsubsetneqq;":           {'\u2ACB', '\uFE00'},
		"varsupsetneq;":            {'\u228B', '\uFE00'},
		"varsupsetneqq;":           {'\u2ACC', '\uFE00'},
		"vnsub;":                   {'\u2282', '\u20D2'},
		"vnsup;":                   {'\u2283', '\u20D2'},
		"vsubnE;":                  {'\u2ACB', '\uFE00'},
		"vsubne;":                  {'\u228A', '\uFE00'},
		"vsupnE;":                  {'\u2ACC', '\uFE00'},
		"vsupne;":                  {'\u228B', '\uFE00'},
	}

	return entity, entity2
})

```

// === FILE: references/go/src/html/escape.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package html provides functions for escaping and unescaping HTML text.
package html

import (
	"strings"
	"unicode/utf8"
)

// These replacements permit compatibility with old numeric entities that
// assumed Windows-1252 encoding.
// https://html.spec.whatwg.org/multipage/parsing.html#numeric-character-reference-end-state
var replacementTable = [...]rune{
	'\u20AC', // First entry is what 0x80 should be replaced with.
	'\u0081',
	'\u201A',
	'\u0192',
	'\u201E',
	'\u2026',
	'\u2020',
	'\u2021',
	'\u02C6',
	'\u2030',
	'\u0160',
	'\u2039',
	'\u0152',
	'\u008D',
	'\u017D',
	'\u008F',
	'\u0090',
	'\u2018',
	'\u2019',
	'\u201C',
	'\u201D',
	'\u2022',
	'\u2013',
	'\u2014',
	'\u02DC',
	'\u2122',
	'\u0161',
	'\u203A',
	'\u0153',
	'\u009D',
	'\u017E',
	'\u0178', // Last entry is 0x9F.
	// 0x00->'\uFFFD' is handled programmatically.
	// 0x0D->'\u000D' is a no-op.
}

// unescapeEntity reads an entity like "&lt;" from b[src:] and writes the
// corresponding "<" to b[dst:], returning the incremented dst and src cursors.
// Precondition: b[src] == '&' && dst <= src.
func unescapeEntity(b []byte, dst, src int, entity map[string]rune, entity2 map[string][2]rune) (dst1, src1 int) {
	const attribute = false

	// http://www.whatwg.org/specs/web-apps/current-work/multipage/tokenization.html#consume-a-character-reference

	// i starts at 1 because we already know that s[0] == '&'.
	i, s := 1, b[src:]

	if len(s) <= 1 {
		b[dst] = b[src]
		return dst + 1, src + 1
	}

	if s[i] == '#' {
		if len(s) <= 3 { // We need to have at least "&#.".
			b[dst] = b[src]
			return dst + 1, src + 1
		}
		i++
		c := s[i]
		hex := false
		if c == 'x' || c == 'X' {
			hex = true
			i++
		}

		x := '\x00'
		for i < len(s) {
			c = s[i]
			i++
			if hex {
				if '0' <= c && c <= '9' {
					x = 16*x + rune(c) - '0'
					continue
				} else if 'a' <= c && c <= 'f' {
					x = 16*x + rune(c) - 'a' + 10
					continue
				} else if 'A' <= c && c <= 'F' {
					x = 16*x + rune(c) - 'A' + 10
					continue
				}
			} else if '0' <= c && c <= '9' {
				x = 10*x + rune(c) - '0'
				continue
			}
			if c != ';' {
				i--
			}
			break
		}

		if i <= 3 { // No characters matched.
			b[dst] = b[src]
			return dst + 1, src + 1
		}

		if 0x80 <= x && x <= 0x9F {
			// Replace characters from Windows-1252 with UTF-8 equivalents.
			x = replacementTable[x-0x80]
		} else if x == 0 || (0xD800 <= x && x <= 0xDFFF) || x > 0x10FFFF {
			// Replace invalid characters with the replacement character.
			x = '\uFFFD'
		}

		return dst + utf8.EncodeRune(b[dst:], x), src + i
	}

	// Consume the maximum number of characters possible, with the
	// consumed characters matching one of the named references.

	for i < len(s) {
		c := s[i]
		i++
		// Lower-cased characters are more common in entities, so we check for them first.
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' {
			continue
		}
		if c != ';' {
			i--
		}
		break
	}

	entityName := s[1:i]
	if len(entityName) == 0 {
		// No-op.
	} else if attribute && entityName[len(entityName)-1] != ';' && len(s) > i && s[i] == '=' {
		// No-op.
	} else if x := entity[string(entityName)]; x != 0 {
		return dst + utf8.EncodeRune(b[dst:], x), src + i
	} else if x := entity2[string(entityName)]; x[0] != 0 {
		dst1 := dst + utf8.EncodeRune(b[dst:], x[0])
		return dst1 + utf8.EncodeRune(b[dst1:], x[1]), src + i
	} else if !attribute {
		maxLen := len(entityName) - 1
		if maxLen > longestEntityWithoutSemicolon {
			maxLen = longestEntityWithoutSemicolon
		}
		for j := maxLen; j > 1; j-- {
			if x := entity[string(entityName[:j])]; x != 0 {
				return dst + utf8.EncodeRune(b[dst:], x), src + j + 1
			}
		}
	}

	dst1, src1 = dst+i, src+i
	copy(b[dst:dst1], b[src:src1])
	return dst1, src1
}

var htmlEscaper = strings.NewReplacer(
	`&`, "&amp;",
	`'`, "&#39;", // "&#39;" is shorter than "&apos;" and apos was not in HTML until HTML5.
	`<`, "&lt;",
	`>`, "&gt;",
	`"`, "&#34;", // "&#34;" is shorter than "&quot;".
)

// EscapeString escapes special characters like "<" to become "&lt;". It
// escapes only five such characters: <, >, &, ' and ".
// [UnescapeString](EscapeString(s)) == s always holds, but the converse isn't
// always true.
func EscapeString(s string) string {
	return htmlEscaper.Replace(s)
}

// UnescapeString unescapes entities like "&lt;" to become "<". It unescapes a
// larger range of entities than [EscapeString] escapes. For example, "&aacute;"
// unescapes to "á", as does "&#225;" and "&#xE1;".
// UnescapeString([EscapeString](s)) == s always holds, but the converse isn't
// always true.
func UnescapeString(s string) string {
	i := strings.IndexByte(s, '&')

	if i < 0 {
		return s
	}

	b := []byte(s)
	entity, entity2 := entityMaps()
	dst, src := unescapeEntity(b, i, i, entity, entity2)
	for len(s[src:]) > 0 {
		if s[src] == '&' {
			i = 0
		} else {
			i = strings.IndexByte(s[src:], '&')
		}
		if i < 0 {
			dst += copy(b[dst:], s[src:])
			break
		}

		if i > 0 {
			copy(b[dst:], s[src:src+i])
		}
		dst, src = unescapeEntity(b, dst+i, src+i, entity, entity2)
	}
	return string(b[:dst])
}

```

// === FILE: references/go/src/html/template/attr.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"strings"
)

// attrTypeMap[n] describes the value of the given attribute.
// If an attribute affects (or can mask) the encoding or interpretation of
// other content, or affects the contents, idempotency, or credentials of a
// network message, then the value in this map is contentTypeUnsafe.
// This map is derived from HTML5, specifically
// https://www.w3.org/TR/html5/Overview.html#attributes-1
// as well as "%URI"-typed attributes from
// https://www.w3.org/TR/html4/index/attributes.html
var attrTypeMap = map[string]contentType{
	"accept":          contentTypePlain,
	"accept-charset":  contentTypeUnsafe,
	"action":          contentTypeURL,
	"alt":             contentTypePlain,
	"archive":         contentTypeURL,
	"async":           contentTypeUnsafe,
	"autocomplete":    contentTypePlain,
	"autofocus":       contentTypePlain,
	"autoplay":        contentTypePlain,
	"background":      contentTypeURL,
	"border":          contentTypePlain,
	"checked":         contentTypePlain,
	"cite":            contentTypeURL,
	"challenge":       contentTypeUnsafe,
	"charset":         contentTypeUnsafe,
	"class":           contentTypePlain,
	"classid":         contentTypeURL,
	"codebase":        contentTypeURL,
	"cols":            contentTypePlain,
	"colspan":         contentTypePlain,
	"content":         contentTypeUnsafe,
	"contenteditable": contentTypePlain,
	"contextmenu":     contentTypePlain,
	"controls":        contentTypePlain,
	"coords":          contentTypePlain,
	"crossorigin":     contentTypeUnsafe,
	"data":            contentTypeURL,
	"datetime":        contentTypePlain,
	"default":         contentTypePlain,
	"defer":           contentTypeUnsafe,
	"dir":             contentTypePlain,
	"dirname":         contentTypePlain,
	"disabled":        contentTypePlain,
	"draggable":       contentTypePlain,
	"dropzone":        contentTypePlain,
	"enctype":         contentTypeUnsafe,
	"for":             contentTypePlain,
	"form":            contentTypeUnsafe,
	"formaction":      contentTypeURL,
	"formenctype":     contentTypeUnsafe,
	"formmethod":      contentTypeUnsafe,
	"formnovalidate":  contentTypeUnsafe,
	"formtarget":      contentTypePlain,
	"headers":         contentTypePlain,
	"height":          contentTypePlain,
	"hidden":          contentTypePlain,
	"high":            contentTypePlain,
	"href":            contentTypeURL,
	"hreflang":        contentTypePlain,
	"http-equiv":      contentTypeUnsafe,
	"icon":            contentTypeURL,
	"id":              contentTypePlain,
	"ismap":           contentTypePlain,
	"keytype":         contentTypeUnsafe,
	"kind":            contentTypePlain,
	"label":           contentTypePlain,
	"lang":            contentTypePlain,
	"language":        contentTypeUnsafe,
	"list":            contentTypePlain,
	"longdesc":        contentTypeURL,
	"loop":            contentTypePlain,
	"low":             contentTypePlain,
	"manifest":        contentTypeURL,
	"max":             contentTypePlain,
	"maxlength":       contentTypePlain,
	"media":           contentTypePlain,
	"mediagroup":      contentTypePlain,
	"method":          contentTypeUnsafe,
	"min":             contentTypePlain,
	"multiple":        contentTypePlain,
	"name":            contentTypePlain,
	"novalidate":      contentTypeUnsafe,
	// Skip handler names from
	// https://www.w3.org/TR/html5/webappapis.html#event-handlers-on-elements,-document-objects,-and-window-objects
	// since we have special handling in attrType.
	"open":        contentTypePlain,
	"optimum":     contentTypePlain,
	"pattern":     contentTypeUnsafe,
	"placeholder": contentTypePlain,
	"poster":      contentTypeURL,
	"profile":     contentTypeURL,
	"preload":     contentTypePlain,
	"pubdate":     contentTypePlain,
	"radiogroup":  contentTypePlain,
	"readonly":    contentTypePlain,
	"rel":         contentTypeUnsafe,
	"required":    contentTypePlain,
	"reversed":    contentTypePlain,
	"rows":        contentTypePlain,
	"rowspan":     contentTypePlain,
	"sandbox":     contentTypeUnsafe,
	"spellcheck":  contentTypePlain,
	"scope":       contentTypePlain,
	"scoped":      contentTypePlain,
	"seamless":    contentTypePlain,
	"selected":    contentTypePlain,
	"shape":       contentTypePlain,
	"size":        contentTypePlain,
	"sizes":       contentTypePlain,
	"span":        contentTypePlain,
	"src":         contentTypeURL,
	"srcdoc":      contentTypeHTML,
	"srclang":     contentTypePlain,
	"srcset":      contentTypeSrcset,
	"start":       contentTypePlain,
	"step":        contentTypePlain,
	"style":       contentTypeCSS,
	"tabindex":    contentTypePlain,
	"target":      contentTypePlain,
	"title":       contentTypePlain,
	"type":        contentTypeUnsafe,
	"usemap":      contentTypeURL,
	"value":       contentTypeUnsafe,
	"width":       contentTypePlain,
	"wrap":        contentTypePlain,
	"xmlns":       contentTypeURL,
}

// attrType returns a conservative (upper-bound on authority) guess at the
// type of the lowercase named attribute.
func attrType(name string) contentType {
	if strings.HasPrefix(name, "data-") {
		// Strip data- so that custom attribute heuristics below are
		// widely applied.
		// Treat data-action as URL below.
		name = name[5:]
	} else if prefix, short, ok := strings.Cut(name, ":"); ok {
		if prefix == "xmlns" {
			return contentTypeURL
		}
		// Treat svg:href and xlink:href as href below.
		name = short
	}
	if t, ok := attrTypeMap[name]; ok {
		return t
	}
	// Treat partial event handler names as script.
	if strings.HasPrefix(name, "on") {
		return contentTypeJS
	}

	// Heuristics to prevent "javascript:..." injection in custom
	// data attributes and custom attributes like g:tweetUrl.
	// https://www.w3.org/TR/html5/dom.html#embedding-custom-non-visible-data-with-the-data-*-attributes
	// "Custom data attributes are intended to store custom data
	//  private to the page or application, for which there are no
	//  more appropriate attributes or elements."
	// Developers seem to store URL content in data URLs that start
	// or end with "URI" or "URL".
	if strings.Contains(name, "src") ||
		strings.Contains(name, "uri") ||
		strings.Contains(name, "url") {
		return contentTypeURL
	}
	return contentTypePlain
}

```

// === FILE: references/go/src/html/template/attr_string.go ===
```go
// Code generated by "stringer -type attr"; DO NOT EDIT.

package template

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[attrNone-0]
	_ = x[attrScript-1]
	_ = x[attrScriptType-2]
	_ = x[attrStyle-3]
	_ = x[attrURL-4]
	_ = x[attrSrcset-5]
	_ = x[attrMetaContent-6]
}

const _attr_name = "attrNoneattrScriptattrScriptTypeattrStyleattrURLattrSrcsetattrMetaContent"

var _attr_index = [...]uint8{0, 8, 18, 32, 41, 48, 58, 73}

func (i attr) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_attr_index)-1 {
		return "attr(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _attr_name[_attr_index[idx]:_attr_index[idx+1]]
}

```

// === FILE: references/go/src/html/template/content.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"reflect"
)

// Strings of content from a trusted source.
type (
	// CSS encapsulates known safe content that matches any of:
	//   1. The CSS3 stylesheet production, such as `p { color: purple }`.
	//   2. The CSS3 rule production, such as `a[href=~"https:"].foo#bar`.
	//   3. CSS3 declaration productions, such as `color: red; margin: 2px`.
	//   4. The CSS3 value production, such as `rgba(0, 0, 255, 127)`.
	// See https://www.w3.org/TR/css3-syntax/#parsing and
	// https://web.archive.org/web/20090211114933/http://w3.org/TR/css3-syntax#style
	//
	// Use of this type presents a security risk:
	// the encapsulated content should come from a trusted source,
	// as it will be included verbatim in the template output.
	CSS string

	// HTML encapsulates a known safe HTML document fragment.
	// It should not be used for HTML from a third-party, or HTML with
	// unclosed tags or comments. The outputs of a sound HTML sanitizer
	// and a template escaped by this package are fine for use with HTML.
	//
	// Use of this type presents a security risk:
	// the encapsulated content should come from a trusted source,
	// as it will be included verbatim in the template output.
	HTML string

	// HTMLAttr encapsulates an HTML attribute from a trusted source,
	// for example, ` dir="ltr"`.
	//
	// Use of this type presents a security risk:
	// the encapsulated content should come from a trusted source,
	// as it will be included verbatim in the template output.
	HTMLAttr string

	// JS encapsulates a known safe EcmaScript5 Expression, for example,
	// `(x + y * z())`.
	// Template authors are responsible for ensuring that typed expressions
	// do not break the intended precedence and that there is no
	// statement/expression ambiguity as when passing an expression like
	// "{ foo: bar() }\n['foo']()", which is both a valid Expression and a
	// valid Program with a very different meaning.
	//
	// Use of this type presents a security risk:
	// the encapsulated content should come from a trusted source,
	// as it will be included verbatim in the template output.
	//
	// Using JS to include valid but untrusted JSON is not safe.
	// A safe alternative is to parse the JSON with json.Unmarshal and then
	// pass the resultant object into the template, where it will be
	// converted to sanitized JSON when presented in a JavaScript context.
	JS string

	// JSStr encapsulates a sequence of characters meant to be embedded
	// between quotes in a JavaScript expression.
	// The string must match a series of StringCharacters:
	//   StringCharacter :: SourceCharacter but not `\` or LineTerminator
	//                    | EscapeSequence
	// Note that LineContinuations are not allowed.
	// JSStr("foo\\nbar") is fine, but JSStr("foo\\\nbar") is not.
	//
	// Use of this type presents a security risk:
	// the encapsulated content should come from a trusted source,
	// as it will be included verbatim in the template output.
	JSStr string

	// URL encapsulates a known safe URL or URL substring (see RFC 3986).
	// A URL like `javascript:checkThatFormNotEditedBeforeLeavingPage()`
	// from a trusted source should go in the page, but by default dynamic
	// `javascript:` URLs are filtered out since they are a frequently
	// exploited injection vector.
	//
	// Use of this type presents a security risk:
	// the encapsulated content should come from a trusted source,
	// as it will be included verbatim in the template output.
	URL string

	// Srcset encapsulates a known safe srcset attribute
	// (see https://w3c.github.io/html/semantics-embedded-content.html#element-attrdef-img-srcset).
	//
	// Use of this type presents a security risk:
	// the encapsulated content should come from a trusted source,
	// as it will be included verbatim in the template output.
	Srcset string
)

type contentType uint8

const (
	contentTypePlain contentType = iota
	contentTypeCSS
	contentTypeHTML
	contentTypeHTMLAttr
	contentTypeJS
	contentTypeJSStr
	contentTypeURL
	contentTypeSrcset
	// contentTypeUnsafe is used in attr.go for values that affect how
	// embedded content and network messages are formed, vetted,
	// or interpreted; or which credentials network messages carry.
	contentTypeUnsafe
)

// indirect returns the value, after dereferencing as many times
// as necessary to reach the base type (or nil).
func indirect(a any) any {
	if a == nil {
		return nil
	}
	if t := reflect.TypeOf(a); t.Kind() != reflect.Pointer {
		// Avoid creating a reflect.Value if it's not a pointer.
		return a
	}
	v := reflect.ValueOf(a)
	for v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	return v.Interface()
}

var (
	errorType       = reflect.TypeFor[error]()
	fmtStringerType = reflect.TypeFor[fmt.Stringer]()
)

// indirectToStringerOrError returns the value, after dereferencing as many times
// as necessary to reach the base type (or nil) or an implementation of fmt.Stringer
// or error.
func indirectToStringerOrError(a any) any {
	if a == nil {
		return nil
	}
	v := reflect.ValueOf(a)
	for !v.Type().Implements(fmtStringerType) && !v.Type().Implements(errorType) && v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	return v.Interface()
}

// stringify converts its arguments to a string and the type of the content.
// All pointers are dereferenced, as in the text/template package.
func stringify(args ...any) (string, contentType) {
	if len(args) == 1 {
		switch s := indirect(args[0]).(type) {
		case string:
			return s, contentTypePlain
		case CSS:
			return string(s), contentTypeCSS
		case HTML:
			return string(s), contentTypeHTML
		case HTMLAttr:
			return string(s), contentTypeHTMLAttr
		case JS:
			return string(s), contentTypeJS
		case JSStr:
			return string(s), contentTypeJSStr
		case URL:
			return string(s), contentTypeURL
		case Srcset:
			return string(s), contentTypeSrcset
		}
	}
	i := 0
	for _, arg := range args {
		// We skip untyped nil arguments for backward compatibility.
		// Without this they would be output as <nil>, escaped.
		// See issue 25875.
		if arg == nil {
			continue
		}

		args[i] = indirectToStringerOrError(arg)
		i++
	}
	return fmt.Sprint(args[:i]...), contentTypePlain
}

```

// === FILE: references/go/src/html/template/context.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"slices"
	"text/template/parse"
)

// context describes the state an HTML parser must be in when it reaches the
// portion of HTML produced by evaluating a particular template node.
//
// The zero value of type context is the start context for a template that
// produces an HTML fragment as defined at
// https://www.w3.org/TR/html5/syntax.html#the-end
// where the context element is null.
type context struct {
	state   state
	delim   delim
	urlPart urlPart
	jsCtx   jsCtx
	// jsBraceDepth contains the current depth, for each JS template literal
	// string interpolation expression, of braces we've seen. This is used to
	// determine if the next } will close a JS template literal string
	// interpolation expression or not.
	jsBraceDepth []int
	attr         attr
	element      element
	n            parse.Node // for range break/continue
	err          *Error
}

func (c context) String() string {
	var err error
	if c.err != nil {
		err = c.err
	}
	return fmt.Sprintf("{%v %v %v %v %v %v %v %v}", c.state, c.delim, c.urlPart, c.jsCtx, c.jsBraceDepth, c.attr, c.element, err)
}

// eq reports whether two contexts are equal.
func (c context) eq(d context) bool {
	return c.state == d.state &&
		c.delim == d.delim &&
		c.urlPart == d.urlPart &&
		c.jsCtx == d.jsCtx &&
		slices.Equal(c.jsBraceDepth, d.jsBraceDepth) &&
		c.attr == d.attr &&
		c.element == d.element &&
		c.err == d.err
}

// mangle produces an identifier that includes a suffix that distinguishes it
// from template names mangled with different contexts.
func (c context) mangle(templateName string) string {
	// The mangled name for the default context is the input templateName.
	if c.state == stateText {
		return templateName
	}
	s := templateName + "$htmltemplate_" + c.state.String()
	if c.delim != delimNone {
		s += "_" + c.delim.String()
	}
	if c.urlPart != urlPartNone {
		s += "_" + c.urlPart.String()
	}
	if c.jsCtx != jsCtxRegexp {
		s += "_" + c.jsCtx.String()
	}
	if c.jsBraceDepth != nil {
		s += fmt.Sprintf("_jsBraceDepth(%v)", c.jsBraceDepth)
	}
	if c.attr != attrNone {
		s += "_" + c.attr.String()
	}
	if c.element != elementNone {
		s += "_" + c.element.String()
	}
	return s
}

// clone returns a copy of c with the same field values.
func (c context) clone() context {
	clone := c
	clone.jsBraceDepth = slices.Clone(c.jsBraceDepth)
	return clone
}

// state describes a high-level HTML parser state.
//
// It bounds the top of the element stack, and by extension the HTML insertion
// mode, but also contains state that does not correspond to anything in the
// HTML5 parsing algorithm because a single token production in the HTML
// grammar may contain embedded actions in a template. For instance, the quoted
// HTML attribute produced by
//
//	<div title="Hello {{.World}}">
//
// is a single token in HTML's grammar but in a template spans several nodes.
type state uint8

//go:generate stringer -type state

const (
	// stateText is parsed character data. An HTML parser is in
	// this state when its parse position is outside an HTML tag,
	// directive, comment, and special element body.
	stateText state = iota
	// stateTag occurs before an HTML attribute or the end of a tag.
	stateTag
	// stateAttrName occurs inside an attribute name.
	// It occurs between the ^'s in ` ^name^ = value`.
	stateAttrName
	// stateAfterName occurs after an attr name has ended but before any
	// equals sign. It occurs between the ^'s in ` name^ ^= value`.
	stateAfterName
	// stateBeforeValue occurs after the equals sign but before the value.
	// It occurs between the ^'s in ` name =^ ^value`.
	stateBeforeValue
	// stateHTMLCmt occurs inside an <!-- HTML comment -->.
	stateHTMLCmt
	// stateRCDATA occurs inside an RCDATA element (<textarea> or <title>)
	// as described at https://www.w3.org/TR/html5/syntax.html#elements-0
	stateRCDATA
	// stateAttr occurs inside an HTML attribute whose content is text.
	stateAttr
	// stateURL occurs inside an HTML attribute whose content is a URL.
	stateURL
	// stateSrcset occurs inside an HTML srcset attribute.
	stateSrcset
	// stateJS occurs inside an event handler or script element.
	stateJS
	// stateJSDqStr occurs inside a JavaScript double quoted string.
	stateJSDqStr
	// stateJSSqStr occurs inside a JavaScript single quoted string.
	stateJSSqStr
	// stateJSTmplLit occurs inside a JavaScript back quoted string.
	stateJSTmplLit
	// stateJSRegexp occurs inside a JavaScript regexp literal.
	stateJSRegexp
	// stateJSBlockCmt occurs inside a JavaScript /* block comment */.
	stateJSBlockCmt
	// stateJSLineCmt occurs inside a JavaScript // line comment.
	stateJSLineCmt
	// stateJSHTMLOpenCmt occurs inside a JavaScript <!-- HTML-like comment.
	stateJSHTMLOpenCmt
	// stateJSHTMLCloseCmt occurs inside a JavaScript --> HTML-like comment.
	stateJSHTMLCloseCmt
	// stateCSS occurs inside a <style> element or style attribute.
	stateCSS
	// stateCSSDqStr occurs inside a CSS double quoted string.
	stateCSSDqStr
	// stateCSSSqStr occurs inside a CSS single quoted string.
	stateCSSSqStr
	// stateCSSDqURL occurs inside a CSS double quoted url("...").
	stateCSSDqURL
	// stateCSSSqURL occurs inside a CSS single quoted url('...').
	stateCSSSqURL
	// stateCSSURL occurs inside a CSS unquoted url(...).
	stateCSSURL
	// stateCSSBlockCmt occurs inside a CSS /* block comment */.
	stateCSSBlockCmt
	// stateCSSLineCmt occurs inside a CSS // line comment.
	stateCSSLineCmt
	// stateError is an infectious error state outside any valid
	// HTML/CSS/JS construct.
	stateError
	// stateMetaContent occurs inside a HTML meta element content attribute.
	stateMetaContent
	// stateMetaContentURL occurs inside a "url=" tag in a HTML meta element content attribute.
	stateMetaContentURL
	// stateDead marks unreachable code after a {{break}} or {{continue}}.
	stateDead
)

// isComment is true for any state that contains content meant for template
// authors & maintainers, not for end-users or machines.
func isComment(s state) bool {
	switch s {
	case stateHTMLCmt, stateJSBlockCmt, stateJSLineCmt, stateJSHTMLOpenCmt, stateJSHTMLCloseCmt, stateCSSBlockCmt, stateCSSLineCmt:
		return true
	}
	return false
}

// isInTag return whether s occurs solely inside an HTML tag.
func isInTag(s state) bool {
	switch s {
	case stateTag, stateAttrName, stateAfterName, stateBeforeValue, stateAttr:
		return true
	}
	return false
}

// isInScriptLiteral returns true if s is one of the literal states within a
// <script> tag, and as such occurrences of "<!--", "<script", and "</script"
// need to be treated specially.
func isInScriptLiteral(s state) bool {
	// Ignore the comment states (stateJSBlockCmt, stateJSLineCmt,
	// stateJSHTMLOpenCmt, stateJSHTMLCloseCmt) because their content is already
	// omitted from the output.
	switch s {
	case stateJSDqStr, stateJSSqStr, stateJSTmplLit, stateJSRegexp:
		return true
	}
	return false
}

// delim is the delimiter that will end the current HTML attribute.
type delim uint8

//go:generate stringer -type delim

const (
	// delimNone occurs outside any attribute.
	delimNone delim = iota
	// delimDoubleQuote occurs when a double quote (") closes the attribute.
	delimDoubleQuote
	// delimSingleQuote occurs when a single quote (') closes the attribute.
	delimSingleQuote
	// delimSpaceOrTagEnd occurs when a space or right angle bracket (>)
	// closes the attribute.
	delimSpaceOrTagEnd
)

// urlPart identifies a part in an RFC 3986 hierarchical URL to allow different
// encoding strategies.
type urlPart uint8

//go:generate stringer -type urlPart

const (
	// urlPartNone occurs when not in a URL, or possibly at the start:
	// ^ in "^http://auth/path?k=v#frag".
	urlPartNone urlPart = iota
	// urlPartPreQuery occurs in the scheme, authority, or path; between the
	// ^s in "h^ttp://auth/path^?k=v#frag".
	urlPartPreQuery
	// urlPartQueryOrFrag occurs in the query portion between the ^s in
	// "http://auth/path?^k=v#frag^".
	urlPartQueryOrFrag
	// urlPartUnknown occurs due to joining of contexts both before and
	// after the query separator.
	urlPartUnknown
)

// jsCtx determines whether a '/' starts a regular expression literal or a
// division operator.
type jsCtx uint8

//go:generate stringer -type jsCtx

const (
	// jsCtxRegexp occurs where a '/' would start a regexp literal.
	jsCtxRegexp jsCtx = iota
	// jsCtxDivOp occurs where a '/' would start a division operator.
	jsCtxDivOp
	// jsCtxUnknown occurs where a '/' is ambiguous due to context joining.
	jsCtxUnknown
)

// element identifies the HTML element when inside a start tag or special body.
// Certain HTML element (for example <script> and <style>) have bodies that are
// treated differently from stateText so the element type is necessary to
// transition into the correct context at the end of a tag and to identify the
// end delimiter for the body.
type element uint8

//go:generate stringer -type element

const (
	// elementNone occurs outside a special tag or special element body.
	elementNone element = iota
	// elementScript corresponds to the raw text <script> element
	// with JS MIME type or no type attribute.
	elementScript
	// elementStyle corresponds to the raw text <style> element.
	elementStyle
	// elementTextarea corresponds to the RCDATA <textarea> element.
	elementTextarea
	// elementTitle corresponds to the RCDATA <title> element.
	elementTitle
	// elementMeta corresponds to the HTML <meta> element.
	elementMeta
)

//go:generate stringer -type attr

// attr identifies the current HTML attribute when inside the attribute,
// that is, starting from stateAttrName until stateTag/stateText (exclusive).
type attr uint8

const (
	// attrNone corresponds to a normal attribute or no attribute.
	attrNone attr = iota
	// attrScript corresponds to an event handler attribute.
	attrScript
	// attrScriptType corresponds to the type attribute in script HTML element
	attrScriptType
	// attrStyle corresponds to the style attribute whose value is CSS.
	attrStyle
	// attrURL corresponds to an attribute whose value is a URL.
	attrURL
	// attrSrcset corresponds to a srcset attribute.
	attrSrcset
	// attrMetaContent corresponds to the content attribute in meta HTML element.
	attrMetaContent
)

```

// === FILE: references/go/src/html/template/css.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// endsWithCSSKeyword reports whether b ends with an ident that
// case-insensitively matches the lower-case kw.
func endsWithCSSKeyword(b []byte, kw string) bool {
	i := len(b) - len(kw)
	if i < 0 {
		// Too short.
		return false
	}
	if i != 0 {
		r, _ := utf8.DecodeLastRune(b[:i])
		if isCSSNmchar(r) {
			// Too long.
			return false
		}
	}
	// Many CSS keywords, such as "!important" can have characters encoded,
	// but the URI production does not allow that according to
	// https://www.w3.org/TR/css3-syntax/#TOK-URI
	// This does not attempt to recognize encoded keywords. For example,
	// given "\75\72\6c" and "url" this return false.
	return string(bytes.ToLower(b[i:])) == kw
}

// isCSSNmchar reports whether rune is allowed anywhere in a CSS identifier.
func isCSSNmchar(r rune) bool {
	// Based on the CSS3 nmchar production but ignores multi-rune escape
	// sequences.
	// https://www.w3.org/TR/css3-syntax/#SUBTOK-nmchar
	return 'a' <= r && r <= 'z' ||
		'A' <= r && r <= 'Z' ||
		'0' <= r && r <= '9' ||
		r == '-' ||
		r == '_' ||
		// Non-ASCII cases below.
		0x80 <= r && r <= 0xd7ff ||
		0xe000 <= r && r <= 0xfffd ||
		0x10000 <= r && r <= 0x10ffff
}

// decodeCSS decodes CSS3 escapes given a sequence of stringchars.
// If there is no change, it returns the input, otherwise it returns a slice
// backed by a new array.
// https://www.w3.org/TR/css3-syntax/#SUBTOK-stringchar defines stringchar.
func decodeCSS(s []byte) []byte {
	i := bytes.IndexByte(s, '\\')
	if i == -1 {
		return s
	}
	// The UTF-8 sequence for a codepoint is never longer than 1 + the
	// number hex digits need to represent that codepoint, so len(s) is an
	// upper bound on the output length.
	b := make([]byte, 0, len(s))
	for len(s) != 0 {
		i := bytes.IndexByte(s, '\\')
		if i == -1 {
			i = len(s)
		}
		b, s = append(b, s[:i]...), s[i:]
		if len(s) < 2 {
			break
		}
		// https://www.w3.org/TR/css3-syntax/#SUBTOK-escape
		// escape ::= unicode | '\' [#x20-#x7E#x80-#xD7FF#xE000-#xFFFD#x10000-#x10FFFF]
		if isHex(s[1]) {
			// https://www.w3.org/TR/css3-syntax/#SUBTOK-unicode
			//   unicode ::= '\' [0-9a-fA-F]{1,6} wc?
			j := 2
			for j < len(s) && j < 7 && isHex(s[j]) {
				j++
			}
			r := hexDecode(s[1:j])
			if r > unicode.MaxRune {
				r, j = r/16, j-1
			}
			n := utf8.EncodeRune(b[len(b):cap(b)], r)
			// The optional space at the end allows a hex
			// sequence to be followed by a literal hex.
			// string(decodeCSS([]byte(`\A B`))) == "\nB"
			b, s = b[:len(b)+n], skipCSSSpace(s[j:])
		} else {
			// `\\` decodes to `\` and `\"` to `"`.
			_, n := utf8.DecodeRune(s[1:])
			b, s = append(b, s[1:1+n]...), s[1+n:]
		}
	}
	return b
}

// isHex reports whether the given character is a hex digit.
func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// hexDecode decodes a short hex digit sequence: "10" -> 16.
func hexDecode(s []byte) rune {
	n := '\x00'
	for _, c := range s {
		n <<= 4
		switch {
		case '0' <= c && c <= '9':
			n |= rune(c - '0')
		case 'a' <= c && c <= 'f':
			n |= rune(c-'a') + 10
		case 'A' <= c && c <= 'F':
			n |= rune(c-'A') + 10
		default:
			panic(fmt.Sprintf("Bad hex digit in %q", s))
		}
	}
	return n
}

// skipCSSSpace returns a suffix of c, skipping over a single space.
func skipCSSSpace(c []byte) []byte {
	if len(c) == 0 {
		return c
	}
	// wc ::= #x9 | #xA | #xC | #xD | #x20
	switch c[0] {
	case '\t', '\n', '\f', ' ':
		return c[1:]
	case '\r':
		// This differs from CSS3's wc production because it contains a
		// probable spec error whereby wc contains all the single byte
		// sequences in nl (newline) but not CRLF.
		if len(c) >= 2 && c[1] == '\n' {
			return c[2:]
		}
		return c[1:]
	}
	return c
}

// isCSSSpace reports whether b is a CSS space char as defined in wc.
func isCSSSpace(b byte) bool {
	switch b {
	case '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

// cssEscaper escapes HTML and CSS special characters using \<hex>+ escapes.
func cssEscaper(args ...any) string {
	s, _ := stringify(args...)
	var b strings.Builder
	r, w, written := rune(0), 0, 0
	for i := 0; i < len(s); i += w {
		// See comment in htmlEscaper.
		r, w = utf8.DecodeRuneInString(s[i:])
		var repl string
		switch {
		case int(r) < len(cssReplacementTable) && cssReplacementTable[r] != "":
			repl = cssReplacementTable[r]
		default:
			continue
		}
		if written == 0 {
			b.Grow(len(s))
		}
		b.WriteString(s[written:i])
		b.WriteString(repl)
		written = i + w
		if repl != `\\` && (written == len(s) || isHex(s[written]) || isCSSSpace(s[written])) {
			b.WriteByte(' ')
		}
	}
	if written == 0 {
		return s
	}
	b.WriteString(s[written:])
	return b.String()
}

var cssReplacementTable = []string{
	0:    `\0`,
	'\t': `\9`,
	'\n': `\a`,
	'\f': `\c`,
	'\r': `\d`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\22`,
	'&':  `\26`,
	'\'': `\27`,
	'(':  `\28`,
	')':  `\29`,
	'+':  `\2b`,
	'/':  `\2f`,
	':':  `\3a`,
	';':  `\3b`,
	'<':  `\3c`,
	'>':  `\3e`,
	'\\': `\\`,
	'{':  `\7b`,
	'}':  `\7d`,
}

var expressionBytes = []byte("expression")
var mozBindingBytes = []byte("mozbinding")

// cssValueFilter allows innocuous CSS values in the output including CSS
// quantities (10px or 25%), ID or class literals (#foo, .bar), keyword values
// (inherit, blue), and colors (#888).
// It filters out unsafe values, such as those that affect token boundaries,
// and anything that might execute scripts.
func cssValueFilter(args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeCSS {
		return s
	}
	b, id := decodeCSS([]byte(s)), make([]byte, 0, 64)

	// CSS3 error handling is specified as honoring string boundaries per
	// https://www.w3.org/TR/css3-syntax/#error-handling :
	//     Malformed declarations. User agents must handle unexpected
	//     tokens encountered while parsing a declaration by reading until
	//     the end of the declaration, while observing the rules for
	//     matching pairs of (), [], {}, "", and '', and correctly handling
	//     escapes. For example, a malformed declaration may be missing a
	//     property, colon (:) or value.
	// So we need to make sure that values do not have mismatched bracket
	// or quote characters to prevent the browser from restarting parsing
	// inside a string that might embed JavaScript source.
	for i, c := range b {
		switch c {
		case 0, '"', '\'', '(', ')', '/', ';', '@', '[', '\\', ']', '`', '{', '}', '<', '>':
			return filterFailsafe
		case '-':
			// Disallow <!-- or -->.
			// -- should not appear in valid identifiers.
			if i != 0 && b[i-1] == '-' {
				return filterFailsafe
			}
		default:
			if c < utf8.RuneSelf && isCSSNmchar(rune(c)) {
				id = append(id, c)
			}
		}
	}
	id = bytes.ToLower(id)
	if bytes.Contains(id, expressionBytes) || bytes.Contains(id, mozBindingBytes) {
		return filterFailsafe
	}
	return string(b)
}

```

// === FILE: references/go/src/html/template/delim_string.go ===
```go
// Code generated by "stringer -type delim"; DO NOT EDIT.

package template

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[delimNone-0]
	_ = x[delimDoubleQuote-1]
	_ = x[delimSingleQuote-2]
	_ = x[delimSpaceOrTagEnd-3]
}

const _delim_name = "delimNonedelimDoubleQuotedelimSingleQuotedelimSpaceOrTagEnd"

var _delim_index = [...]uint8{0, 9, 25, 41, 59}

func (i delim) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_delim_index)-1 {
		return "delim(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _delim_name[_delim_index[idx]:_delim_index[idx+1]]
}

```

// === FILE: references/go/src/html/template/doc.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package template (html/template) implements data-driven templates for
generating HTML output safe against code injection. It provides the
same interface as [text/template] and should be used instead of
[text/template] whenever the output is HTML.

The documentation here focuses on the security features of the package.
For information about how to program the templates themselves, see the
documentation for [text/template].

# Introduction

This package wraps [text/template] so you can share its template API
to parse and execute HTML templates safely.

	tmpl, err := template.New("name").Parse(...)
	// Error checking elided
	err = tmpl.Execute(out, data)

If successful, tmpl will now be injection-safe. Otherwise, err is an error
defined in the docs for ErrorCode.

HTML templates treat data values as plain text which should be encoded so they
can be safely embedded in an HTML document. The escaping is contextual, so
actions can appear within JavaScript, CSS, and URI contexts.

Comments are stripped from output, except for those passed in via the
[HTML], [CSS], and [JS] types for their respective contexts.

The security model used by this package assumes that template authors are
trusted, while Execute's data parameter is not. More details are
provided below.

Example

	import "text/template"
	...
	t, err := template.New("foo").Parse(`{{define "T"}}Hello, {{.}}!{{end}}`)
	err = t.ExecuteTemplate(out, "T", "<script>alert('you have been pwned')</script>")

produces

	Hello, <script>alert('you have been pwned')</script>!

but the contextual autoescaping in html/template

	import "html/template"
	...
	t, err := template.New("foo").Parse(`{{define "T"}}Hello, {{.}}!{{end}}`)
	err = t.ExecuteTemplate(out, "T", "<script>alert('you have been pwned')</script>")

produces safe, escaped HTML output

	Hello, &lt;script&gt;alert(&#39;you have been pwned&#39;)&lt;/script&gt;!

# Contexts

This package understands HTML, CSS, JavaScript, and URIs. It adds sanitizing
functions to each simple action pipeline, so given the excerpt

	<a href="/search?q={{.}}">{{.}}</a>

At parse time each {{.}} is overwritten to add escaping functions as necessary.
In this case it becomes

	<a href="/search?q={{. | urlescaper | attrescaper}}">{{. | htmlescaper}}</a>

where urlescaper, attrescaper, and htmlescaper are aliases for internal escaping
functions.

For these internal escaping functions, if an action pipeline evaluates to
a nil interface value, it is treated as though it were an empty string.

# Namespaced and data- attributes

Attributes with a namespace are treated as if they had no namespace.
Given the excerpt

	<a my:href="{{.}}"></a>

At parse time the attribute will be treated as if it were just "href".
So at parse time the template becomes:

	<a my:href="{{. | urlescaper | attrescaper}}"></a>

Similarly to attributes with namespaces, attributes with a "data-" prefix are
treated as if they had no "data-" prefix. So given

	<a data-href="{{.}}"></a>

At parse time this becomes

	<a data-href="{{. | urlescaper | attrescaper}}"></a>

If an attribute has both a namespace and a "data-" prefix, only the namespace
will be removed when determining the context. For example

	<a my:data-href="{{.}}"></a>

This is handled as if "my:data-href" was just "data-href" and not "href" as
it would be if the "data-" prefix were to be ignored too. Thus at parse
time this becomes just

	<a my:data-href="{{. | attrescaper}}"></a>

As a special case, attributes with the namespace "xmlns" are always treated
as containing URLs. Given the excerpts

	<a xmlns:title="{{.}}"></a>
	<a xmlns:href="{{.}}"></a>
	<a xmlns:onclick="{{.}}"></a>

At parse time they become:

	<a xmlns:title="{{. | urlescaper | attrescaper}}"></a>
	<a xmlns:href="{{. | urlescaper | attrescaper}}"></a>
	<a xmlns:onclick="{{. | urlescaper | attrescaper}}"></a>

# Errors

See the documentation of ErrorCode for details.

# A fuller picture

The rest of this package comment may be skipped on first reading; it includes
details necessary to understand escaping contexts and error messages. Most users
will not need to understand these details.

# Contexts

Assuming {{.}} is `O'Reilly: How are <i>you</i>?`, the table below shows
how {{.}} appears when used in the context to the left.

	Context                          {{.}} After
	{{.}}                            O'Reilly: How are &lt;i&gt;you&lt;/i&gt;?
	<a title='{{.}}'>                O&#39;Reilly: How are you?
	<a href="/{{.}}">                O&#39;Reilly: How are %3ci%3eyou%3c/i%3e?
	<a href="?q={{.}}">              O&#39;Reilly%3a%20How%20are%3ci%3e...%3f
	<a onx='f("{{.}}")'>             O\x27Reilly: How are \x3ci\x3eyou...?
	<a onx='f({{.}})'>               "O\x27Reilly: How are \x3ci\x3eyou...?"
	<a onx='pattern = /{{.}}/;'>     O\x27Reilly: How are \x3ci\x3eyou...\x3f

If used in an unsafe context, then the value might be filtered out:

	Context                          {{.}} After
	<a href="{{.}}">                 #ZgotmplZ

since "O'Reilly:" is not an allowed protocol like "http:".

If {{.}} is the innocuous word, `left`, then it can appear more widely,

	Context                              {{.}} After
	{{.}}                                left
	<a title='{{.}}'>                    left
	<a href='{{.}}'>                     left
	<a href='/{{.}}'>                    left
	<a href='?dir={{.}}'>                left
	<a style="border-{{.}}: 4px">        left
	<a style="align: {{.}}">             left
	<a style="background: '{{.}}'>       left
	<a style="background: url('{{.}}')>  left
	<style>p.{{.}} {color:red}</style>   left

Non-string values can be used in JavaScript contexts.
If {{.}} is

	struct{A,B string}{ "foo", "bar" }

in the escaped template

	<script>var pair = {{.}};</script>

then the template output is

	<script>var pair = {"A": "foo", "B": "bar"};</script>

See package json to understand how non-string content is marshaled for
embedding in JavaScript contexts.

# Typed Strings

By default, this package assumes that all pipelines produce a plain text string.
It adds escaping pipeline stages necessary to correctly and safely embed that
plain text string in the appropriate context.

When a data value is not plain text, you can make sure it is not over-escaped
by marking it with its type.

Types HTML, JS, URL, and others from content.go can carry safe content that is
exempted from escaping.

The template

	Hello, {{.}}!

can be invoked with

	tmpl.Execute(out, template.HTML(`<b>World</b>`))

to produce

	Hello, <b>World</b>!

instead of the

	Hello, &lt;b&gt;World&lt;b&gt;!

that would have been produced if {{.}} was a regular string.

# Security Model

https://web.archive.org/web/20160501113828/http://js-quasis-libraries-and-repl.googlecode.com/svn/trunk/safetemplate.html#problem_definition defines "safe" as used by this package.

This package assumes that template authors are trusted, that Execute's data
parameter is not, and seeks to preserve the properties below in the face
of untrusted data:

Structure Preservation Property:
"... when a template author writes an HTML tag in a safe templating language,
the browser will interpret the corresponding portion of the output as a tag
regardless of the values of untrusted data, and similarly for other structures
such as attribute boundaries and JS and CSS string boundaries."

Code Effect Property:
"... only code specified by the template author should run as a result of
injecting the template output into a page and all code specified by the
template author should run as a result of the same."

Least Surprise Property:
"A developer (or code reviewer) familiar with HTML, CSS, and JavaScript, who
knows that contextual autoescaping happens should be able to look at a {{.}}
and correctly infer what sanitization happens."

Previously, ECMAScript 6 template literal were disabled by default, and could be
enabled with the GODEBUG=jstmpllitinterp=1 environment variable. Template
literals are now supported by default, and setting jstmpllitinterp has no
effect.
*/
package template

```

// === FILE: references/go/src/html/template/element_string.go ===
```go
// Code generated by "stringer -type element"; DO NOT EDIT.

package template

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[elementNone-0]
	_ = x[elementScript-1]
	_ = x[elementStyle-2]
	_ = x[elementTextarea-3]
	_ = x[elementTitle-4]
	_ = x[elementMeta-5]
}

const _element_name = "elementNoneelementScriptelementStyleelementTextareaelementTitleelementMeta"

var _element_index = [...]uint8{0, 11, 24, 36, 51, 63, 74}

func (i element) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_element_index)-1 {
		return "element(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _element_name[_element_index[idx]:_element_index[idx+1]]
}

```

// === FILE: references/go/src/html/template/error.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"text/template/parse"
)

// Error describes a problem encountered during template Escaping.
type Error struct {
	// ErrorCode describes the kind of error.
	ErrorCode ErrorCode
	// Node is the node that caused the problem, if known.
	// If not nil, it overrides Name and Line.
	Node parse.Node
	// Name is the name of the template in which the error was encountered.
	Name string
	// Line is the line number of the error in the template source or 0.
	Line int
	// Description is a human-readable description of the problem.
	Description string
}

// ErrorCode is a code for a kind of error.
type ErrorCode int

// We define codes for each error that manifests while escaping templates, but
// escaped templates may also fail at runtime.
//
// Output: "ZgotmplZ"
// Example:
//
//	<img src="{{.X}}">
//	where {{.X}} evaluates to `javascript:...`
//
// Discussion:
//
//	"ZgotmplZ" is a special value that indicates that unsafe content reached a
//	CSS or URL context at runtime. The output of the example will be
//	  <img src="#ZgotmplZ">
//	If the data comes from a trusted source, use content types to exempt it
//	from filtering: URL(`javascript:...`).
const (
	// OK indicates the lack of an error.
	OK ErrorCode = iota

	// ErrAmbigContext: "... appears in an ambiguous context within a URL"
	// Example:
	//   <a href="
	//      {{if .C}}
	//        /path/
	//      {{else}}
	//        /search?q=
	//      {{end}}
	//      {{.X}}
	//   ">
	// Discussion:
	//   {{.X}} is in an ambiguous URL context since, depending on {{.C}},
	//  it may be either a URL suffix or a query parameter.
	//   Moving {{.X}} into the condition removes the ambiguity:
	//   <a href="{{if .C}}/path/{{.X}}{{else}}/search?q={{.X}}">
	ErrAmbigContext

	// ErrBadHTML: "expected space, attr name, or end of tag, but got ...",
	//   "... in unquoted attr", "... in attribute name"
	// Example:
	//   <a href = /search?q=foo>
	//   <href=foo>
	//   <form na<e=...>
	//   <option selected<
	// Discussion:
	//   This is often due to a typo in an HTML element, but some runes
	//   are banned in tag names, attribute names, and unquoted attribute
	//   values because they can tickle parser ambiguities.
	//   Quoting all attributes is the best policy.
	ErrBadHTML

	// ErrBranchEnd: "{{if}} branches end in different contexts"
	// Examples:
	//   {{if .C}}<a href="{{end}}{{.X}}
	//   <script {{with .T}}type="{{.}}"{{end}}>
	// Discussion:
	//   Package html/template statically examines each path through an
	//   {{if}}, {{range}}, or {{with}} to escape any following pipelines.
	//   The first example is ambiguous since {{.X}} might be an HTML text node,
	//   or a URL prefix in an HTML attribute. The context of {{.X}} is
	//   used to figure out how to escape it, but that context depends on
	//   the run-time value of {{.C}} which is not statically known.
	//   The second example is ambiguous as the script type attribute
	//   can change the type of escaping needed for the script contents.
	//
	//   The problem is usually something like missing quotes or angle
	//   brackets, or can be avoided by refactoring to put the two contexts
	//   into different branches of an if, range or with. If the problem
	//   is in a {{range}} over a collection that should never be empty,
	//   adding a dummy {{else}} can help.
	ErrBranchEnd

	// ErrEndContext: "... ends in a non-text context: ..."
	// Examples:
	//   <div
	//   <div title="no close quote>
	//   <script>f()
	// Discussion:
	//   Executed templates should produce a DocumentFragment of HTML.
	//   Templates that end without closing tags will trigger this error.
	//   Templates that should not be used in an HTML context or that
	//   produce incomplete Fragments should not be executed directly.
	//
	//   {{define "main"}} <script>{{template "helper"}}</script> {{end}}
	//   {{define "helper"}} document.write(' <div title=" ') {{end}}
	//
	//   "helper" does not produce a valid document fragment, so should
	//   not be Executed directly.
	ErrEndContext

	// ErrNoSuchTemplate: "no such template ..."
	// Examples:
	//   {{define "main"}}<div {{template "attrs"}}>{{end}}
	//   {{define "attrs"}}href="{{.URL}}"{{end}}
	// Discussion:
	//   Package html/template looks through template calls to compute the
	//   context.
	//   Here the {{.URL}} in "attrs" must be treated as a URL when called
	//   from "main", but you will get this error if "attrs" is not defined
	//   when "main" is parsed.
	ErrNoSuchTemplate

	// ErrOutputContext: "cannot compute output context for template ..."
	// Examples:
	//   {{define "t"}}{{if .T}}{{template "t" .T}}{{end}}{{.H}}",{{end}}
	// Discussion:
	//   A recursive template does not end in the same context in which it
	//   starts, and a reliable output context cannot be computed.
	//   Look for typos in the named template.
	//   If the template should not be called in the named start context,
	//   look for calls to that template in unexpected contexts.
	//   Maybe refactor recursive templates to not be recursive.
	ErrOutputContext

	// ErrPartialCharset: "unfinished JS regexp charset in ..."
	// Example:
	//     <script>var pattern = /foo[{{.Chars}}]/</script>
	// Discussion:
	//   Package html/template does not support interpolation into regular
	//   expression literal character sets.
	ErrPartialCharset

	// ErrPartialEscape: "unfinished escape sequence in ..."
	// Example:
	//   <script>alert("\{{.X}}")</script>
	// Discussion:
	//   Package html/template does not support actions following a
	//   backslash.
	//   This is usually an error and there are better solutions; for
	//   example
	//     <script>alert("{{.X}}")</script>
	//   should work, and if {{.X}} is a partial escape sequence such as
	//   "xA0", mark the whole sequence as safe content: JSStr(`\xA0`)
	ErrPartialEscape

	// ErrRangeLoopReentry: "on range loop re-entry: ..."
	// Example:
	//   <script>var x = [{{range .}}'{{.}},{{end}}]</script>
	// Discussion:
	//   If an iteration through a range would cause it to end in a
	//   different context than an earlier pass, there is no single context.
	//   In the example, there is missing a quote, so it is not clear
	//   whether {{.}} is meant to be inside a JS string or in a JS value
	//   context. The second iteration would produce something like
	//
	//     <script>var x = ['firstValue,'secondValue]</script>
	ErrRangeLoopReentry

	// ErrSlashAmbig: '/' could start a division or regexp.
	// Example:
	//   <script>
	//     {{if .C}}var x = 1{{end}}
	//     /-{{.N}}/i.test(x) ? doThis : doThat();
	//   </script>
	// Discussion:
	//   The example above could produce `var x = 1/-2/i.test(s)...`
	//   in which the first '/' is a mathematical division operator or it
	//   could produce `/-2/i.test(s)` in which the first '/' starts a
	//   regexp literal.
	//   Look for missing semicolons inside branches, and maybe add
	//   parentheses to make it clear which interpretation you intend.
	ErrSlashAmbig

	// ErrPredefinedEscaper: "predefined escaper ... disallowed in template"
	// Example:
	//   <div class={{. | html}}>Hello<div>
	// Discussion:
	//   Package html/template already contextually escapes all pipelines to
	//   produce HTML output safe against code injection. Manually escaping
	//   pipeline output using the predefined escapers "html" or "urlquery" is
	//   unnecessary, and may affect the correctness or safety of the escaped
	//   pipeline output in Go 1.8 and earlier.
	//
	//   In most cases, such as the given example, this error can be resolved by
	//   simply removing the predefined escaper from the pipeline and letting the
	//   contextual autoescaper handle the escaping of the pipeline. In other
	//   instances, where the predefined escaper occurs in the middle of a
	//   pipeline where subsequent commands expect escaped input, e.g.
	//     {{.X | html | makeALink}}
	//   where makeALink does
	//     return `<a href="`+input+`">link</a>`
	//   consider refactoring the surrounding template to make use of the
	//   contextual autoescaper, i.e.
	//     <a href="{{.X}}">link</a>
	//
	//   To ease migration to Go 1.9 and beyond, "html" and "urlquery" will
	//   continue to be allowed as the last command in a pipeline. However, if the
	//   pipeline occurs in an unquoted attribute value context, "html" is
	//   disallowed. Avoid using "html" and "urlquery" entirely in new templates.
	ErrPredefinedEscaper

	// ErrJSTemplate: "... appears in a JS template literal"
	// Example:
	//     <script>var tmpl = `{{.Interp}}`</script>
	// Discussion:
	//   Package html/template does not support actions inside of JS template
	//   literals.
	//
	// Deprecated: ErrJSTemplate is no longer returned when an action is present
	// in a JS template literal. Actions inside of JS template literals are now
	// escaped as expected.
	ErrJSTemplate
)

func (e *Error) Error() string {
	switch {
	case e.Node != nil:
		loc, _ := (*parse.Tree)(nil).ErrorContext(e.Node)
		return fmt.Sprintf("html/template:%s: %s", loc, e.Description)
	case e.Line != 0:
		return fmt.Sprintf("html/template:%s:%d: %s", e.Name, e.Line, e.Description)
	case e.Name != "":
		return fmt.Sprintf("html/template:%s: %s", e.Name, e.Description)
	}
	return "html/template: " + e.Description
}

// errorf creates an error given a format string f and args.
// The template Name still needs to be supplied.
func errorf(k ErrorCode, node parse.Node, line int, f string, args ...any) *Error {
	return &Error{k, node, "", line, fmt.Sprintf(f, args...)}
}

```

// === FILE: references/go/src/html/template/escape.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"fmt"
	"html"
	"internal/godebug"
	"io"
	"maps"
	"regexp"
	"text/template"
	"text/template/parse"
)

// escapeTemplate rewrites the named template, which must be
// associated with t, to guarantee that the output of any of the named
// templates is properly escaped. If no error is returned, then the named templates have
// been modified. Otherwise the named templates have been rendered
// unusable.
func escapeTemplate(tmpl *Template, node parse.Node, name string) error {
	c, _ := tmpl.esc.escapeTree(context{}, node, name, 0)
	var err error
	if c.err != nil {
		err, c.err.Name = c.err, name
	} else if c.state != stateText {
		err = &Error{ErrEndContext, nil, name, 0, fmt.Sprintf("ends in a non-text context: %v", c)}
	}
	if err != nil {
		// Prevent execution of unsafe templates.
		if t := tmpl.set[name]; t != nil {
			t.escapeErr = err
			t.text.Tree = nil
			t.Tree = nil
		}
		return err
	}
	tmpl.esc.commit()
	if t := tmpl.set[name]; t != nil {
		t.escapeErr = escapeOK
		t.Tree = t.text.Tree
	}
	return nil
}

// evalArgs formats the list of arguments into a string. It is equivalent to
// fmt.Sprint(args...), except that it dereferences all pointers.
func evalArgs(args ...any) string {
	// Optimization for simple common case of a single string argument.
	if len(args) == 1 {
		if s, ok := args[0].(string); ok {
			return s
		}
	}
	for i, arg := range args {
		args[i] = indirectToStringerOrError(arg)
	}
	return fmt.Sprint(args...)
}

// funcMap maps command names to functions that render their inputs safe.
var funcMap = template.FuncMap{
	"_html_template_attrescaper":      attrEscaper,
	"_html_template_commentescaper":   commentEscaper,
	"_html_template_cssescaper":       cssEscaper,
	"_html_template_cssvaluefilter":   cssValueFilter,
	"_html_template_htmlnamefilter":   htmlNameFilter,
	"_html_template_htmlescaper":      htmlEscaper,
	"_html_template_jsregexpescaper":  jsRegexpEscaper,
	"_html_template_jsstrescaper":     jsStrEscaper,
	"_html_template_jstmpllitescaper": jsTmplLitEscaper,
	"_html_template_jsvalescaper":     jsValEscaper,
	"_html_template_nospaceescaper":   htmlNospaceEscaper,
	"_html_template_rcdataescaper":    rcdataEscaper,
	"_html_template_srcsetescaper":    srcsetFilterAndEscaper,
	"_html_template_urlescaper":       urlEscaper,
	"_html_template_urlfilter":        urlFilter,
	"_html_template_urlnormalizer":    urlNormalizer,
	"_eval_args_":                     evalArgs,
}

// escaper collects type inferences about templates and changes needed to make
// templates injection safe.
type escaper struct {
	// ns is the nameSpace that this escaper is associated with.
	ns *nameSpace
	// output[templateName] is the output context for a templateName that
	// has been mangled to include its input context.
	output map[string]context
	// derived[c.mangle(name)] maps to a template derived from the template
	// named name templateName for the start context c.
	derived map[string]*template.Template
	// called[templateName] is a set of called mangled template names.
	called map[string]bool
	// xxxNodeEdits are the accumulated edits to apply during commit.
	// Such edits are not applied immediately in case a template set
	// executes a given template in different escaping contexts.
	actionNodeEdits   map[*parse.ActionNode][]string
	templateNodeEdits map[*parse.TemplateNode]string
	textNodeEdits     map[*parse.TextNode][]byte
	// rangeContext holds context about the current range loop.
	rangeContext *rangeContext
}

// rangeContext holds information about the current range loop.
type rangeContext struct {
	outer     *rangeContext // outer loop
	breaks    []context     // context at each break action
	continues []context     // context at each continue action
}

// makeEscaper creates a blank escaper for the given set.
func makeEscaper(n *nameSpace) escaper {
	return escaper{
		n,
		map[string]context{},
		map[string]*template.Template{},
		map[string]bool{},
		map[*parse.ActionNode][]string{},
		map[*parse.TemplateNode]string{},
		map[*parse.TextNode][]byte{},
		nil,
	}
}

// filterFailsafe is an innocuous word that is emitted in place of unsafe values
// by sanitizer functions. It is not a keyword in any programming language,
// contains no special characters, is not empty, and when it appears in output
// it is distinct enough that a developer can find the source of the problem
// via a search engine.
const filterFailsafe = "ZgotmplZ"

// escape escapes a template node.
func (e *escaper) escape(c context, n parse.Node) context {
	switch n := n.(type) {
	case *parse.ActionNode:
		return e.escapeAction(c, n)
	case *parse.BreakNode:
		c.n = n
		e.rangeContext.breaks = append(e.rangeContext.breaks, c)
		return context{state: stateDead}
	case *parse.CommentNode:
		return c
	case *parse.ContinueNode:
		c.n = n
		e.rangeContext.continues = append(e.rangeContext.continues, c)
		return context{state: stateDead}
	case *parse.IfNode:
		return e.escapeBranch(c, &n.BranchNode, "if")
	case *parse.ListNode:
		return e.escapeList(c, n)
	case *parse.RangeNode:
		return e.escapeBranch(c, &n.BranchNode, "range")
	case *parse.TemplateNode:
		return e.escapeTemplate(c, n)
	case *parse.TextNode:
		return e.escapeText(c, n)
	case *parse.WithNode:
		return e.escapeBranch(c, &n.BranchNode, "with")
	}
	panic("escaping " + n.String() + " is unimplemented")
}

var debugAllowActionJSTmpl = godebug.New("jstmpllitinterp")

var htmlmetacontenturlescape = godebug.New("htmlmetacontenturlescape")

// escapeAction escapes an action template node.
func (e *escaper) escapeAction(c context, n *parse.ActionNode) context {
	if len(n.Pipe.Decl) != 0 {
		// A local variable assignment, not an interpolation.
		return c
	}
	c = nudge(c)
	// Check for disallowed use of predefined escapers in the pipeline.
	for pos, idNode := range n.Pipe.Cmds {
		node, ok := idNode.Args[0].(*parse.IdentifierNode)
		if !ok {
			// A predefined escaper "esc" will never be found as an identifier in a
			// Chain or Field node, since:
			// - "esc.x ..." is invalid, since predefined escapers return strings, and
			//   strings do not have methods, keys or fields.
			// - "... .esc" is invalid, since predefined escapers are global functions,
			//   not methods or fields of any types.
			// Therefore, it is safe to ignore these two node types.
			continue
		}
		ident := node.Ident
		if _, ok := predefinedEscapers[ident]; ok {
			if pos < len(n.Pipe.Cmds)-1 ||
				c.state == stateAttr && c.delim == delimSpaceOrTagEnd && ident == "html" {
				return context{
					state: stateError,
					err:   errorf(ErrPredefinedEscaper, n, n.Line, "predefined escaper %q disallowed in template", ident),
				}
			}
		}
	}
	s := make([]string, 0, 3)
	switch c.state {
	case stateError:
		return c
	case stateURL, stateCSSDqStr, stateCSSSqStr, stateCSSDqURL, stateCSSSqURL, stateCSSURL:
		switch c.urlPart {
		case urlPartNone:
			s = append(s, "_html_template_urlfilter")
			fallthrough
		case urlPartPreQuery:
			switch c.state {
			case stateCSSDqStr, stateCSSSqStr:
				s = append(s, "_html_template_cssescaper")
			default:
				s = append(s, "_html_template_urlnormalizer")
			}
		case urlPartQueryOrFrag:
			s = append(s, "_html_template_urlescaper")
		case urlPartUnknown:
			return context{
				state: stateError,
				err:   errorf(ErrAmbigContext, n, n.Line, "%s appears in an ambiguous context within a URL", n),
			}
		default:
			panic(c.urlPart.String())
		}
	case stateMetaContent:
		// Handled below in delim check.
	case stateMetaContentURL:
		if htmlmetacontenturlescape.Value() != "0" {
			s = append(s, "_html_template_urlfilter")
		} else {
			// We don't have a great place to increment this, since it's hard to
			// know if we actually escape any urls in _html_template_urlfilter,
			// since it has no information about what context it is being
			// executed in etc. This is probably the best we can do.
			htmlmetacontenturlescape.IncNonDefault()
		}
	case stateJS:
		s = append(s, "_html_template_jsvalescaper")
		// A slash after a value starts a div operator.
		c.jsCtx = jsCtxDivOp
	case stateJSDqStr, stateJSSqStr:
		s = append(s, "_html_template_jsstrescaper")
	case stateJSTmplLit:
		s = append(s, "_html_template_jstmpllitescaper")
	case stateJSRegexp:
		s = append(s, "_html_template_jsregexpescaper")
	case stateCSS:
		s = append(s, "_html_template_cssvaluefilter")
	case stateText:
		s = append(s, "_html_template_htmlescaper")
	case stateRCDATA:
		s = append(s, "_html_template_rcdataescaper")
	case stateAttr:
		// Handled below in delim check.
	case stateAttrName, stateTag:
		c.state = stateAttrName
		s = append(s, "_html_template_htmlnamefilter")
	case stateSrcset:
		s = append(s, "_html_template_srcsetescaper")
	default:
		if isComment(c.state) {
			s = append(s, "_html_template_commentescaper")
		} else {
			panic("unexpected state " + c.state.String())
		}
	}
	switch c.delim {
	case delimNone:
		// No extra-escaping needed for raw text content.
	case delimSpaceOrTagEnd:
		s = append(s, "_html_template_nospaceescaper")
	default:
		s = append(s, "_html_template_attrescaper")
	}
	e.editActionNode(n, s)
	return c
}

// ensurePipelineContains ensures that the pipeline ends with the commands with
// the identifiers in s in order. If the pipeline ends with a predefined escaper
// (i.e. "html" or "urlquery"), merge it with the identifiers in s.
func ensurePipelineContains(p *parse.PipeNode, s []string) {
	if len(s) == 0 {
		// Do not rewrite pipeline if we have no escapers to insert.
		return
	}
	// Precondition: p.Cmds contains at most one predefined escaper and the
	// escaper will be present at p.Cmds[len(p.Cmds)-1]. This precondition is
	// always true because of the checks in escapeAction.
	pipelineLen := len(p.Cmds)
	if pipelineLen > 0 {
		lastCmd := p.Cmds[pipelineLen-1]
		if idNode, ok := lastCmd.Args[0].(*parse.IdentifierNode); ok {
			if esc := idNode.Ident; predefinedEscapers[esc] {
				// Pipeline ends with a predefined escaper.
				if len(p.Cmds) == 1 && len(lastCmd.Args) > 1 {
					// Special case: pipeline is of the form {{ esc arg1 arg2 ... argN }},
					// where esc is the predefined escaper, and arg1...argN are its arguments.
					// Convert this into the equivalent form
					// {{ _eval_args_ arg1 arg2 ... argN | esc }}, so that esc can be easily
					// merged with the escapers in s.
					lastCmd.Args[0] = parse.NewIdentifier("_eval_args_").SetTree(nil).SetPos(lastCmd.Args[0].Position())
					p.Cmds = appendCmd(p.Cmds, newIdentCmd(esc, p.Position()))
					pipelineLen++
				}
				// If any of the commands in s that we are about to insert is equivalent
				// to the predefined escaper, use the predefined escaper instead.
				dup := false
				for i, escaper := range s {
					if escFnsEq(esc, escaper) {
						s[i] = idNode.Ident
						dup = true
					}
				}
				if dup {
					// The predefined escaper will already be inserted along with the
					// escapers in s, so do not copy it to the rewritten pipeline.
					pipelineLen--
				}
			}
		}
	}
	// Rewrite the pipeline, creating the escapers in s at the end of the pipeline.
	newCmds := make([]*parse.CommandNode, pipelineLen, pipelineLen+len(s))
	insertedIdents := make(map[string]bool)
	for i := 0; i < pipelineLen; i++ {
		cmd := p.Cmds[i]
		newCmds[i] = cmd
		if idNode, ok := cmd.Args[0].(*parse.IdentifierNode); ok {
			insertedIdents[normalizeEscFn(idNode.Ident)] = true
		}
	}
	for _, name := range s {
		if !insertedIdents[normalizeEscFn(name)] {
			// When two templates share an underlying parse tree via the use of
			// AddParseTree and one template is executed after the other, this check
			// ensures that escapers that were already inserted into the pipeline on
			// the first escaping pass do not get inserted again.
			newCmds = appendCmd(newCmds, newIdentCmd(name, p.Position()))
		}
	}
	p.Cmds = newCmds
}

// predefinedEscapers contains template predefined escapers that are equivalent
// to some contextual escapers. Keep in sync with equivEscapers.
var predefinedEscapers = map[string]bool{
	"html":     true,
	"urlquery": true,
}

// equivEscapers matches contextual escapers to equivalent predefined
// template escapers.
var equivEscapers = map[string]string{
	// The following pairs of HTML escapers provide equivalent security
	// guarantees, since they all escape '\000', '\'', '"', '&', '<', and '>'.
	"_html_template_attrescaper":   "html",
	"_html_template_htmlescaper":   "html",
	"_html_template_rcdataescaper": "html",
	// These two URL escapers produce URLs safe for embedding in a URL query by
	// percent-encoding all the reserved characters specified in RFC 3986 Section
	// 2.2
	"_html_template_urlescaper": "urlquery",
	// These two functions are not actually equivalent; urlquery is stricter as it
	// escapes reserved characters (e.g. '#'), while _html_template_urlnormalizer
	// does not. It is therefore only safe to replace _html_template_urlnormalizer
	// with urlquery (this happens in ensurePipelineContains), but not the otherI've
	// way around. We keep this entry around to preserve the behavior of templates
	// written before Go 1.9, which might depend on this substitution taking place.
	"_html_template_urlnormalizer": "urlquery",
}

// escFnsEq reports whether the two escaping functions are equivalent.
func escFnsEq(a, b string) bool {
	return normalizeEscFn(a) == normalizeEscFn(b)
}

// normalizeEscFn(a) is equal to normalizeEscFn(b) for any pair of names of
// escaper functions a and b that are equivalent.
func normalizeEscFn(e string) string {
	if norm := equivEscapers[e]; norm != "" {
		return norm
	}
	return e
}

// redundantFuncs[a][b] implies that funcMap[b](funcMap[a](x)) == funcMap[a](x)
// for all x.
var redundantFuncs = map[string]map[string]bool{
	"_html_template_commentescaper": {
		"_html_template_attrescaper": true,
		"_html_template_htmlescaper": true,
	},
	"_html_template_cssescaper": {
		"_html_template_attrescaper": true,
	},
	"_html_template_jsregexpescaper": {
		"_html_template_attrescaper": true,
	},
	"_html_template_jsstrescaper": {
		"_html_template_attrescaper": true,
	},
	"_html_template_jstmpllitescaper": {
		"_html_template_attrescaper": true,
	},
	"_html_template_urlescaper": {
		"_html_template_urlnormalizer": true,
	},
}

// appendCmd appends the given command to the end of the command pipeline
// unless it is redundant with the last command.
func appendCmd(cmds []*parse.CommandNode, cmd *parse.CommandNode) []*parse.CommandNode {
	if n := len(cmds); n != 0 {
		last, okLast := cmds[n-1].Args[0].(*parse.IdentifierNode)
		next, okNext := cmd.Args[0].(*parse.IdentifierNode)
		if okLast && okNext && redundantFuncs[last.Ident][next.Ident] {
			return cmds
		}
	}
	return append(cmds, cmd)
}

// newIdentCmd produces a command containing a single identifier node.
func newIdentCmd(identifier string, pos parse.Pos) *parse.CommandNode {
	return &parse.CommandNode{
		NodeType: parse.NodeCommand,
		Args:     []parse.Node{parse.NewIdentifier(identifier).SetTree(nil).SetPos(pos)}, // TODO: SetTree.
	}
}

// nudge returns the context that would result from following empty string
// transitions from the input context.
// For example, parsing:
//
//	`<a href=`
//
// will end in context{stateBeforeValue, attrURL}, but parsing one extra rune:
//
//	`<a href=x`
//
// will end in context{stateURL, delimSpaceOrTagEnd, ...}.
// There are two transitions that happen when the 'x' is seen:
// (1) Transition from a before-value state to a start-of-value state without
//
//	consuming any character.
//
// (2) Consume 'x' and transition past the first value character.
// In this case, nudging produces the context after (1) happens.
func nudge(c context) context {
	switch c.state {
	case stateTag:
		// In `<foo {{.}}`, the action should emit an attribute.
		c.state = stateAttrName
	case stateBeforeValue:
		// In `<foo bar={{.}}`, the action is an undelimited value.
		c.state, c.delim, c.attr = attrStartStates[c.attr], delimSpaceOrTagEnd, attrNone
	case stateAfterName:
		// In `<foo bar {{.}}`, the action is an attribute name.
		c.state, c.attr = stateAttrName, attrNone
	}
	return c
}

// join joins the two contexts of a branch template node. The result is an
// error context if either of the input contexts are error contexts, or if the
// input contexts differ.
func join(a, b context, node parse.Node, nodeName string) context {
	if a.state == stateError {
		return a
	}
	if b.state == stateError {
		return b
	}
	if a.state == stateDead {
		return b
	}
	if b.state == stateDead {
		return a
	}
	if a.eq(b) {
		return a
	}

	c := a
	c.urlPart = b.urlPart
	if c.eq(b) {
		// The contexts differ only by urlPart.
		c.urlPart = urlPartUnknown
		return c
	}

	c = a
	c.jsCtx = b.jsCtx
	if c.eq(b) {
		// The contexts differ only by jsCtx.
		c.jsCtx = jsCtxUnknown
		return c
	}

	// Allow a nudged context to join with an unnudged one.
	// This means that
	//   <p title={{if .C}}{{.}}{{end}}
	// ends in an unquoted value state even though the else branch
	// ends in stateBeforeValue.
	if c, d := nudge(a), nudge(b); !(c.eq(a) && d.eq(b)) {
		if e := join(c, d, node, nodeName); e.state != stateError {
			return e
		}
	}

	return context{
		state: stateError,
		err:   errorf(ErrBranchEnd, node, 0, "{{%s}} branches end in different contexts: %v, %v", nodeName, a, b),
	}
}

// escapeBranch escapes a branch template node: "if", "range" and "with".
func (e *escaper) escapeBranch(c context, n *parse.BranchNode, nodeName string) context {
	if nodeName == "range" {
		e.rangeContext = &rangeContext{outer: e.rangeContext}
	}
	c0 := e.escapeList(c.clone(), n.List)
	if nodeName == "range" {
		if c0.state != stateError {
			c0 = joinRange(c0, e.rangeContext)
		}
		e.rangeContext = e.rangeContext.outer
		if c0.state == stateError {
			return c0
		}

		// The "true" branch of a "range" node can execute multiple times.
		// We check that executing n.List once results in the same context
		// as executing n.List twice.
		e.rangeContext = &rangeContext{outer: e.rangeContext}
		c1, _ := e.escapeListConditionally(c0, n.List, nil)
		c0 = join(c0, c1, n, nodeName)
		if c0.state == stateError {
			e.rangeContext = e.rangeContext.outer
			// Make clear that this is a problem on loop re-entry
			// since developers tend to overlook that branch when
			// debugging templates.
			c0.err.Line = n.Line
			c0.err.Description = "on range loop re-entry: " + c0.err.Description
			return c0
		}
		c0 = joinRange(c0, e.rangeContext)
		e.rangeContext = e.rangeContext.outer
		if c0.state == stateError {
			return c0
		}
	}
	c1 := e.escapeList(c.clone(), n.ElseList)
	return join(c0, c1, n, nodeName)
}

func joinRange(c0 context, rc *rangeContext) context {
	// Merge contexts at break and continue statements into overall body context.
	// In theory we could treat breaks differently from continues, but for now it is
	// enough to treat them both as going back to the start of the loop (which may then stop).
	for _, c := range rc.breaks {
		c0 = join(c0, c, c.n, "range")
		if c0.state == stateError {
			c0.err.Line = c.n.(*parse.BreakNode).Line
			c0.err.Description = "at range loop break: " + c0.err.Description
			return c0
		}
	}
	for _, c := range rc.continues {
		c0 = join(c0, c, c.n, "range")
		if c0.state == stateError {
			c0.err.Line = c.n.(*parse.ContinueNode).Line
			c0.err.Description = "at range loop continue: " + c0.err.Description
			return c0
		}
	}
	return c0
}

// escapeList escapes a list template node.
func (e *escaper) escapeList(c context, n *parse.ListNode) context {
	if n == nil {
		return c
	}
	for _, m := range n.Nodes {
		c = e.escape(c, m)
		if c.state == stateDead {
			break
		}
	}
	return c
}

// escapeListConditionally escapes a list node but only preserves edits and
// inferences in e if the inferences and output context satisfy filter.
// It returns the best guess at an output context, and the result of the filter
// which is the same as whether e was updated.
func (e *escaper) escapeListConditionally(c context, n *parse.ListNode, filter func(*escaper, context) bool) (context, bool) {
	e1 := makeEscaper(e.ns)
	e1.rangeContext = e.rangeContext
	// Make type inferences available to f.
	maps.Copy(e1.output, e.output)
	c = e1.escapeList(c, n)
	ok := filter != nil && filter(&e1, c)
	if ok {
		// Copy inferences and edits from e1 back into e.
		maps.Copy(e.output, e1.output)
		maps.Copy(e.derived, e1.derived)
		maps.Copy(e.called, e1.called)
		for k, v := range e1.actionNodeEdits {
			e.editActionNode(k, v)
		}
		for k, v := range e1.templateNodeEdits {
			e.editTemplateNode(k, v)
		}
		for k, v := range e1.textNodeEdits {
			e.editTextNode(k, v)
		}
	}
	return c, ok
}

// escapeTemplate escapes a {{template}} call node.
func (e *escaper) escapeTemplate(c context, n *parse.TemplateNode) context {
	c, name := e.escapeTree(c, n, n.Name, n.Line)
	if name != n.Name {
		e.editTemplateNode(n, name)
	}
	return c
}

// escapeTree escapes the named template starting in the given context as
// necessary and returns its output context.
func (e *escaper) escapeTree(c context, node parse.Node, name string, line int) (context, string) {
	// Mangle the template name with the input context to produce a reliable
	// identifier.
	dname := c.mangle(name)
	e.called[dname] = true
	if out, ok := e.output[dname]; ok {
		// Already escaped.
		return out, dname
	}
	t := e.template(name)
	if t == nil {
		// Two cases: The template exists but is empty, or has never been mentioned at
		// all. Distinguish the cases in the error messages.
		if e.ns.set[name] != nil {
			return context{
				state: stateError,
				err:   errorf(ErrNoSuchTemplate, node, line, "%q is an incomplete or empty template", name),
			}, dname
		}
		return context{
			state: stateError,
			err:   errorf(ErrNoSuchTemplate, node, line, "no such template %q", name),
		}, dname
	}
	if dname != name {
		// Use any template derived during an earlier call to escapeTemplate
		// with different top level templates, or clone if necessary.
		dt := e.template(dname)
		if dt == nil {
			dt = template.New(dname)
			dt.Tree = &parse.Tree{Name: dname, Root: t.Root.CopyList()}
			e.derived[dname] = dt
		}
		t = dt
	}
	return e.computeOutCtx(c, t), dname
}

// computeOutCtx takes a template and its start context and computes the output
// context while storing any inferences in e.
func (e *escaper) computeOutCtx(c context, t *template.Template) context {
	// Propagate context over the body.
	c1, ok := e.escapeTemplateBody(c, t)
	if !ok {
		// Look for a fixed point by assuming c1 as the output context.
		if c2, ok2 := e.escapeTemplateBody(c1, t); ok2 {
			c1, ok = c2, true
		}
		// Use c1 as the error context if neither assumption worked.
	}
	if !ok && c1.state != stateError {
		return context{
			state: stateError,
			err:   errorf(ErrOutputContext, t.Tree.Root, 0, "cannot compute output context for template %s", t.Name()),
		}
	}
	return c1
}

// escapeTemplateBody escapes the given template assuming the given output
// context, and returns the best guess at the output context and whether the
// assumption was correct.
func (e *escaper) escapeTemplateBody(c context, t *template.Template) (context, bool) {
	filter := func(e1 *escaper, c1 context) bool {
		if c1.state == stateError {
			// Do not update the input escaper, e.
			return false
		}
		if !e1.called[t.Name()] {
			// If t is not recursively called, then c1 is an
			// accurate output context.
			return true
		}
		// c1 is accurate if it matches our assumed output context.
		return c.eq(c1)
	}
	// We need to assume an output context so that recursive template calls
	// take the fast path out of escapeTree instead of infinitely recurring.
	// Naively assuming that the input context is the same as the output
	// works >90% of the time.
	e.output[t.Name()] = c
	return e.escapeListConditionally(c, t.Tree.Root, filter)
}

// delimEnds maps each delim to a string of characters that terminate it.
var delimEnds = [...]string{
	delimDoubleQuote: `"`,
	delimSingleQuote: "'",
	// Determined empirically by running the below in various browsers.
	// var div = document.createElement("DIV");
	// for (var i = 0; i < 0x10000; ++i) {
	//   div.innerHTML = "<span title=x" + String.fromCharCode(i) + "-bar>";
	//   if (div.getElementsByTagName("SPAN")[0].title.indexOf("bar") < 0)
	//     document.write("<p>U+" + i.toString(16));
	// }
	delimSpaceOrTagEnd: " \t\n\f\r>",
}

var (
	// Per WHATWG HTML specification, section 4.12.1.3, there are extremely
	// complicated rules for how to handle the set of opening tags <!--,
	// <script, and </script when they appear in JS literals (i.e. strings,
	// regexs, and comments). The specification suggests a simple solution,
	// rather than implementing the arcane ABNF, which involves simply escaping
	// the opening bracket with \x3C. We use the below regex for this, since it
	// makes doing the case-insensitive find-replace much simpler.
	specialScriptTagRE          = regexp.MustCompile("(?i)<(script|/script|!--)")
	specialScriptTagReplacement = []byte("\\x3C$1")
)

func containsSpecialScriptTag(s []byte) bool {
	return specialScriptTagRE.Match(s)
}

func escapeSpecialScriptTags(s []byte) []byte {
	return specialScriptTagRE.ReplaceAll(s, specialScriptTagReplacement)
}

var doctypeBytes = []byte("<!DOCTYPE")

// escapeText escapes a text template node.
func (e *escaper) escapeText(c context, n *parse.TextNode) context {
	s, written, i, b := n.Text, 0, 0, new(bytes.Buffer)
	for i != len(s) {
		c1, nread := contextAfterText(c, s[i:])
		i1 := i + nread
		if c.state == stateText || c.state == stateRCDATA {
			end := i1
			if c1.state != c.state {
				for j := end - 1; j >= i; j-- {
					if s[j] == '<' {
						end = j
						break
					}
				}
			}
			for j := i; j < end; j++ {
				if s[j] == '<' && !bytes.HasPrefix(bytes.ToUpper(s[j:]), doctypeBytes) {
					b.Write(s[written:j])
					b.WriteString("&lt;")
					written = j + 1
				}
			}
		} else if isComment(c.state) && c.delim == delimNone {
			switch c.state {
			case stateJSBlockCmt:
				// https://es5.github.io/#x7.4:
				// "Comments behave like white space and are
				// discarded except that, if a MultiLineComment
				// contains a line terminator character, then
				// the entire comment is considered to be a
				// LineTerminator for purposes of parsing by
				// the syntactic grammar."
				if bytes.ContainsAny(s[written:i1], "\n\r\u2028\u2029") {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
			case stateCSSBlockCmt:
				b.WriteByte(' ')
			}
			written = i1
		}
		if c.state != c1.state && isComment(c1.state) && c1.delim == delimNone {
			// Preserve the portion between written and the comment start.
			cs := i1 - 2
			if c1.state == stateHTMLCmt || c1.state == stateJSHTMLOpenCmt {
				// "<!--" instead of "/*" or "//"
				cs -= 2
			} else if c1.state == stateJSHTMLCloseCmt {
				// "-->" instead of "/*" or "//"
				cs -= 1
			}
			b.Write(s[written:cs])
			written = i1
		}
		if isInScriptLiteral(c.state) && containsSpecialScriptTag(s[i:i1]) {
			b.Write(s[written:i])
			b.Write(escapeSpecialScriptTags(s[i:i1]))
			written = i1
		}
		if i == i1 && c.state == c1.state {
			panic(fmt.Sprintf("infinite loop from %v to %v on %q..%q", c, c1, s[:i], s[i:]))
		}
		c, i = c1, i1
	}

	if written != 0 && c.state != stateError {
		if !isComment(c.state) || c.delim != delimNone {
			b.Write(n.Text[written:])
		}
		e.editTextNode(n, b.Bytes())
	}
	return c
}

// contextAfterText starts in context c, consumes some tokens from the front of
// s, then returns the context after those tokens and the unprocessed suffix.
func contextAfterText(c context, s []byte) (context, int) {
	if c.delim == delimNone {
		c1, i := tSpecialTagEnd(c, s)
		if i == 0 {
			// A special end tag (`</script>`) has been seen and
			// all content preceding it has been consumed.
			return c1, 0
		}
		// Consider all content up to any end tag.
		return transitionFunc[c.state](c, s[:i])
	}

	// We are at the beginning of an attribute value.

	i := bytes.IndexAny(s, delimEnds[c.delim])
	if i == -1 {
		i = len(s)
	}
	if c.delim == delimSpaceOrTagEnd {
		// https://www.w3.org/TR/html5/syntax.html#attribute-value-(unquoted)-state
		// lists the runes below as error characters.
		// Error out because HTML parsers may differ on whether
		// "<a id= onclick=f("     ends inside id's or onclick's value,
		// "<a class=`foo "        ends inside a value,
		// "<a style=font:'Arial'" needs open-quote fixup.
		// IE treats '`' as a quotation character.
		if j := bytes.IndexAny(s[:i], "\"'<=`"); j >= 0 {
			return context{
				state: stateError,
				err:   errorf(ErrBadHTML, nil, 0, "%q in unquoted attr: %q", s[j:j+1], s[:i]),
			}, len(s)
		}
	}
	if i == len(s) {
		// Remain inside the attribute.
		// Decode the value so non-HTML rules can easily handle
		//     <button onclick="alert(&quot;Hi!&quot;)">
		// without having to entity decode token boundaries.
		for u := []byte(html.UnescapeString(string(s))); len(u) != 0; {
			c1, i1 := transitionFunc[c.state](c, u)
			c, u = c1, u[i1:]
		}
		return c, len(s)
	}

	element := c.element

	// If this is a non-JS "type" attribute inside "script" tag, do not treat the contents as JS.
	if c.state == stateAttr && c.element == elementScript && c.attr == attrScriptType && !isJSType(string(s[:i])) {
		element = elementNone
	}

	if c.delim != delimSpaceOrTagEnd {
		// Consume any quote.
		i++
	}
	// On exiting an attribute, we discard all state information
	// except the state and element.
	return context{state: stateTag, element: element}, i
}

// editActionNode records a change to an action pipeline for later commit.
func (e *escaper) editActionNode(n *parse.ActionNode, cmds []string) {
	if _, ok := e.actionNodeEdits[n]; ok {
		panic(fmt.Sprintf("node %s shared between templates", n))
	}
	e.actionNodeEdits[n] = cmds
}

// editTemplateNode records a change to a {{template}} callee for later commit.
func (e *escaper) editTemplateNode(n *parse.TemplateNode, callee string) {
	if _, ok := e.templateNodeEdits[n]; ok {
		panic(fmt.Sprintf("node %s shared between templates", n))
	}
	e.templateNodeEdits[n] = callee
}

// editTextNode records a change to a text node for later commit.
func (e *escaper) editTextNode(n *parse.TextNode, text []byte) {
	if _, ok := e.textNodeEdits[n]; ok {
		panic(fmt.Sprintf("node %s shared between templates", n))
	}
	e.textNodeEdits[n] = text
}

// commit applies changes to actions and template calls needed to contextually
// autoescape content and adds any derived templates to the set.
func (e *escaper) commit() {
	for name := range e.output {
		e.template(name).Funcs(funcMap)
	}
	// Any template from the name space associated with this escaper can be used
	// to add derived templates to the underlying text/template name space.
	tmpl := e.arbitraryTemplate()
	for _, t := range e.derived {
		if _, err := tmpl.text.AddParseTree(t.Name(), t.Tree); err != nil {
			panic("error adding derived template")
		}
	}
	for n, s := range e.actionNodeEdits {
		ensurePipelineContains(n.Pipe, s)
	}
	for n, name := range e.templateNodeEdits {
		n.Name = name
	}
	for n, s := range e.textNodeEdits {
		n.Text = s
	}
	// Reset state that is specific to this commit so that the same changes are
	// not re-applied to the template on subsequent calls to commit.
	e.called = make(map[string]bool)
	e.actionNodeEdits = make(map[*parse.ActionNode][]string)
	e.templateNodeEdits = make(map[*parse.TemplateNode]string)
	e.textNodeEdits = make(map[*parse.TextNode][]byte)
}

// template returns the named template given a mangled template name.
func (e *escaper) template(name string) *template.Template {
	// Any template from the name space associated with this escaper can be used
	// to look up templates in the underlying text/template name space.
	t := e.arbitraryTemplate().text.Lookup(name)
	if t == nil {
		t = e.derived[name]
	}
	return t
}

// arbitraryTemplate returns an arbitrary template from the name space
// associated with e and panics if no templates are found.
func (e *escaper) arbitraryTemplate() *Template {
	for _, t := range e.ns.set {
		return t
	}
	panic("no templates in name space")
}

// Forwarding functions so that clients need only import this package
// to reach the general escaping functions of text/template.

// HTMLEscape writes to w the escaped HTML equivalent of the plain text data b.
func HTMLEscape(w io.Writer, b []byte) {
	template.HTMLEscape(w, b)
}

// HTMLEscapeString returns the escaped HTML equivalent of the plain text data s.
func HTMLEscapeString(s string) string {
	return template.HTMLEscapeString(s)
}

// HTMLEscaper returns the escaped HTML equivalent of the textual
// representation of its arguments.
func HTMLEscaper(args ...any) string {
	return template.HTMLEscaper(args...)
}

// JSEscape writes to w the escaped JavaScript equivalent of the plain text data b.
func JSEscape(w io.Writer, b []byte) {
	template.JSEscape(w, b)
}

// JSEscapeString returns the escaped JavaScript equivalent of the plain text data s.
func JSEscapeString(s string) string {
	return template.JSEscapeString(s)
}

// JSEscaper returns the escaped JavaScript equivalent of the textual
// representation of its arguments.
func JSEscaper(args ...any) string {
	return template.JSEscaper(args...)
}

// URLQueryEscaper returns the escaped value of the textual representation of
// its arguments in a form suitable for embedding in a URL query.
func URLQueryEscaper(args ...any) string {
	return template.URLQueryEscaper(args...)
}

```

// === FILE: references/go/src/html/template/html.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// htmlNospaceEscaper escapes for inclusion in unquoted attribute values.
func htmlNospaceEscaper(args ...any) string {
	s, t := stringify(args...)
	if s == "" {
		return filterFailsafe
	}
	if t == contentTypeHTML {
		return htmlReplacer(stripTags(s), htmlNospaceNormReplacementTable, false)
	}
	return htmlReplacer(s, htmlNospaceReplacementTable, false)
}

// attrEscaper escapes for inclusion in quoted attribute values.
func attrEscaper(args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeHTML {
		return htmlReplacer(stripTags(s), htmlNormReplacementTable, true)
	}
	return htmlReplacer(s, htmlReplacementTable, true)
}

// rcdataEscaper escapes for inclusion in an RCDATA element body.
func rcdataEscaper(args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeHTML {
		return htmlReplacer(s, htmlNormReplacementTable, true)
	}
	return htmlReplacer(s, htmlReplacementTable, true)
}

// htmlEscaper escapes for inclusion in HTML text.
func htmlEscaper(args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeHTML {
		return s
	}
	return htmlReplacer(s, htmlReplacementTable, true)
}

// htmlReplacementTable contains the runes that need to be escaped
// inside a quoted attribute value or in a text node.
var htmlReplacementTable = []string{
	// https://www.w3.org/TR/html5/syntax.html#attribute-value-(unquoted)-state
	// U+0000 NULL Parse error. Append a U+FFFD REPLACEMENT
	// CHARACTER character to the current attribute's value.
	// "
	// and similarly
	// https://www.w3.org/TR/html5/syntax.html#before-attribute-value-state
	0:    "\uFFFD",
	'"':  "&#34;",
	'&':  "&amp;",
	'\'': "&#39;",
	'+':  "&#43;",
	'<':  "&lt;",
	'>':  "&gt;",
}

// htmlNormReplacementTable is like htmlReplacementTable but without '&' to
// avoid over-encoding existing entities.
var htmlNormReplacementTable = []string{
	0:    "\uFFFD",
	'"':  "&#34;",
	'\'': "&#39;",
	'+':  "&#43;",
	'<':  "&lt;",
	'>':  "&gt;",
}

// htmlNospaceReplacementTable contains the runes that need to be escaped
// inside an unquoted attribute value.
// The set of runes escaped is the union of the HTML specials and
// those determined by running the JS below in browsers:
// <div id=d></div>
// <script>(function () {
// var a = [], d = document.getElementById("d"), i, c, s;
// for (i = 0; i < 0x10000; ++i) {
//
//	c = String.fromCharCode(i);
//	d.innerHTML = "<span title=" + c + "lt" + c + "></span>"
//	s = d.getElementsByTagName("SPAN")[0];
//	if (!s || s.title !== c + "lt" + c) { a.push(i.toString(16)); }
//
// }
// document.write(a.join(", "));
// })()</script>
var htmlNospaceReplacementTable = []string{
	0:    "&#xfffd;",
	'\t': "&#9;",
	'\n': "&#10;",
	'\v': "&#11;",
	'\f': "&#12;",
	'\r': "&#13;",
	' ':  "&#32;",
	'"':  "&#34;",
	'&':  "&amp;",
	'\'': "&#39;",
	'+':  "&#43;",
	'<':  "&lt;",
	'=':  "&#61;",
	'>':  "&gt;",
	// A parse error in the attribute value (unquoted) and
	// before attribute value states.
	// Treated as a quoting character by IE.
	'`': "&#96;",
}

// htmlNospaceNormReplacementTable is like htmlNospaceReplacementTable but
// without '&' to avoid over-encoding existing entities.
var htmlNospaceNormReplacementTable = []string{
	0:    "&#xfffd;",
	'\t': "&#9;",
	'\n': "&#10;",
	'\v': "&#11;",
	'\f': "&#12;",
	'\r': "&#13;",
	' ':  "&#32;",
	'"':  "&#34;",
	'\'': "&#39;",
	'+':  "&#43;",
	'<':  "&lt;",
	'=':  "&#61;",
	'>':  "&gt;",
	// A parse error in the attribute value (unquoted) and
	// before attribute value states.
	// Treated as a quoting character by IE.
	'`': "&#96;",
}

// htmlReplacer returns s with runes replaced according to replacementTable
// and when badRunes is true, certain bad runes are allowed through unescaped.
func htmlReplacer(s string, replacementTable []string, badRunes bool) string {
	written, b := 0, new(strings.Builder)
	r, w := rune(0), 0
	for i := 0; i < len(s); i += w {
		// Cannot use 'for range s' because we need to preserve the width
		// of the runes in the input. If we see a decoding error, the input
		// width will not be utf8.Runelen(r) and we will overrun the buffer.
		r, w = utf8.DecodeRuneInString(s[i:])
		if int(r) < len(replacementTable) {
			if repl := replacementTable[r]; len(repl) != 0 {
				if written == 0 {
					b.Grow(len(s))
				}
				b.WriteString(s[written:i])
				b.WriteString(repl)
				written = i + w
			}
		} else if badRunes {
			// No-op.
			// IE does not allow these ranges in unquoted attrs.
		} else if 0xfdd0 <= r && r <= 0xfdef || 0xfff0 <= r && r <= 0xffff {
			if written == 0 {
				b.Grow(len(s))
			}
			fmt.Fprintf(b, "%s&#x%x;", s[written:i], r)
			written = i + w
		}
	}
	if written == 0 {
		return s
	}
	b.WriteString(s[written:])
	return b.String()
}

// stripTags takes a snippet of HTML and returns only the text content.
// For example, `<b>&iexcl;Hi!</b> <script>...</script>` -> `&iexcl;Hi! `.
func stripTags(html string) string {
	var b strings.Builder
	s, c, i, allText := []byte(html), context{}, 0, true
	// Using the transition funcs helps us avoid mangling
	// `<div title="1>2">` or `I <3 Ponies!`.
	for i != len(s) {
		if c.delim == delimNone {
			st := c.state
			// Use RCDATA instead of parsing into JS or CSS styles.
			if c.element != elementNone && !isInTag(st) {
				st = stateRCDATA
			}
			d, nread := transitionFunc[st](c, s[i:])
			i1 := i + nread
			if c.state == stateText || c.state == stateRCDATA {
				// Emit text up to the start of the tag or comment.
				j := i1
				if d.state != c.state {
					for j1 := j - 1; j1 >= i; j1-- {
						if s[j1] == '<' {
							j = j1
							break
						}
					}
				}
				b.Write(s[i:j])
			} else {
				allText = false
			}
			c, i = d, i1
			continue
		}
		i1 := i + bytes.IndexAny(s[i:], delimEnds[c.delim])
		if i1 < i {
			break
		}
		if c.delim != delimSpaceOrTagEnd {
			// Consume any quote.
			i1++
		}
		c, i = context{state: stateTag, element: c.element}, i1
	}
	if allText {
		return html
	} else if c.state == stateText || c.state == stateRCDATA {
		b.Write(s[i:])
	}
	return b.String()
}

// htmlNameFilter accepts valid parts of an HTML attribute or tag name or
// a known-safe HTML attribute.
func htmlNameFilter(args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeHTMLAttr {
		return s
	}
	if len(s) == 0 {
		// Avoid violation of structure preservation.
		// <input checked {{.K}}={{.V}}>.
		// Without this, if .K is empty then .V is the value of
		// checked, but otherwise .V is the value of the attribute
		// named .K.
		return filterFailsafe
	}
	s = strings.ToLower(s)
	if t := attrType(s); t != contentTypePlain {
		// TODO: Split attr and element name part filters so we can recognize known attributes.
		return filterFailsafe
	}
	for _, r := range s {
		switch {
		case '0' <= r && r <= '9':
		case 'a' <= r && r <= 'z':
		default:
			return filterFailsafe
		}
	}
	return s
}

// commentEscaper returns the empty string regardless of input.
// Comment content does not correspond to any parsed structure or
// human-readable content, so the simplest and most secure policy is to drop
// content interpolated into comments.
// This approach is equally valid whether or not static comment content is
// removed from the template.
func commentEscaper(args ...any) string {
	return ""
}

```

// === FILE: references/go/src/html/template/js.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
)

// jsWhitespace contains all of the JS whitespace characters, as defined
// by the \s character class.
// See https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_expressions/Character_classes.
const jsWhitespace = "\f\n\r\t\v\u0020\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"

// nextJSCtx returns the context that determines whether a slash after the
// given run of tokens starts a regular expression instead of a division
// operator: / or /=.
//
// This assumes that the token run does not include any string tokens, comment
// tokens, regular expression literal tokens, or division operators.
//
// This fails on some valid but nonsensical JavaScript programs like
// "x = ++/foo/i" which is quite different than "x++/foo/i", but is not known to
// fail on any known useful programs. It is based on the draft
// JavaScript 2.0 lexical grammar and requires one token of lookbehind:
// https://www.mozilla.org/js/language/js20-2000-07/rationale/syntax.html
func nextJSCtx(s []byte, preceding jsCtx) jsCtx {
	// Trim all JS whitespace characters
	s = bytes.TrimRight(s, jsWhitespace)
	if len(s) == 0 {
		return preceding
	}

	// All cases below are in the single-byte UTF-8 group.
	switch c, n := s[len(s)-1], len(s); c {
	case '+', '-':
		// ++ and -- are not regexp preceders, but + and - are whether
		// they are used as infix or prefix operators.
		start := n - 1
		// Count the number of adjacent dashes or pluses.
		for start > 0 && s[start-1] == c {
			start--
		}
		if (n-start)&1 == 1 {
			// Reached for trailing minus signs since "---" is the
			// same as "-- -".
			return jsCtxRegexp
		}
		return jsCtxDivOp
	case '.':
		// Handle "42."
		if n != 1 && '0' <= s[n-2] && s[n-2] <= '9' {
			return jsCtxDivOp
		}
		return jsCtxRegexp
	// Suffixes for all punctuators from section 7.7 of the language spec
	// that only end binary operators not handled above.
	case ',', '<', '>', '=', '*', '%', '&', '|', '^', '?':
		return jsCtxRegexp
	// Suffixes for all punctuators from section 7.7 of the language spec
	// that are prefix operators not handled above.
	case '!', '~':
		return jsCtxRegexp
	// Matches all the punctuators from section 7.7 of the language spec
	// that are open brackets not handled above.
	case '(', '[':
		return jsCtxRegexp
	// Matches all the punctuators from section 7.7 of the language spec
	// that precede expression starts.
	case ':', ';', '{':
		return jsCtxRegexp
	// CAVEAT: the close punctuators ('}', ']', ')') precede div ops and
	// are handled in the default except for '}' which can precede a
	// division op as in
	//    ({ valueOf: function () { return 42 } } / 2
	// which is valid, but, in practice, developers don't divide object
	// literals, so our heuristic works well for code like
	//    function () { ... }  /foo/.test(x) && sideEffect();
	// The ')' punctuator can precede a regular expression as in
	//     if (b) /foo/.test(x) && ...
	// but this is much less likely than
	//     (a + b) / c
	case '}':
		return jsCtxRegexp
	default:
		// Look for an IdentifierName and see if it is a keyword that
		// can precede a regular expression.
		j := n
		for j > 0 && isJSIdentPart(rune(s[j-1])) {
			j--
		}
		if regexpPrecederKeywords[string(s[j:])] {
			return jsCtxRegexp
		}
	}
	// Otherwise is a punctuator not listed above, or
	// a string which precedes a div op, or an identifier
	// which precedes a div op.
	return jsCtxDivOp
}

// regexpPrecederKeywords is a set of reserved JS keywords that can precede a
// regular expression in JS source.
var regexpPrecederKeywords = map[string]bool{
	"break":      true,
	"case":       true,
	"continue":   true,
	"delete":     true,
	"do":         true,
	"else":       true,
	"finally":    true,
	"in":         true,
	"instanceof": true,
	"return":     true,
	"throw":      true,
	"try":        true,
	"typeof":     true,
	"void":       true,
}

var jsonMarshalType = reflect.TypeFor[json.Marshaler]()

// indirectToJSONMarshaler returns the value, after dereferencing as many times
// as necessary to reach the base type (or nil) or an implementation of json.Marshal.
func indirectToJSONMarshaler(a any) any {
	// text/template now supports passing untyped nil as a func call
	// argument, so we must support it. Otherwise we'd panic below, as one
	// cannot call the Type or Interface methods on an invalid
	// reflect.Value. See golang.org/issue/18716.
	if a == nil {
		return nil
	}

	v := reflect.ValueOf(a)
	for !v.Type().Implements(jsonMarshalType) && v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	return v.Interface()
}

var scriptTagRe = regexp.MustCompile("(?i)<(/?)script")

// jsValEscaper escapes its inputs to a JS Expression (section 11.14) that has
// neither side-effects nor free variables outside (NaN, Infinity).
func jsValEscaper(args ...any) string {
	var a any
	if len(args) == 1 {
		a = indirectToJSONMarshaler(args[0])
		switch t := a.(type) {
		case JS:
			return string(t)
		case JSStr:
			// TODO: normalize quotes.
			return `"` + string(t) + `"`
		case json.Marshaler:
			// Do not treat as a Stringer.
		case fmt.Stringer:
			a = t.String()
		}
	} else {
		for i, arg := range args {
			args[i] = indirectToJSONMarshaler(arg)
		}
		a = fmt.Sprint(args...)
	}
	// TODO: detect cycles before calling Marshal which loops infinitely on
	// cyclic data. This may be an unacceptable DoS risk.
	b, err := json.Marshal(a)
	if err != nil {
		// While the standard JSON marshaler does not include user controlled
		// information in the error message, if a type has a MarshalJSON method,
		// the content of the error message is not guaranteed. Since we insert
		// the error into the template, as part of a comment, we attempt to
		// prevent the error from either terminating the comment, or the script
		// block itself.
		//
		// In particular we:
		//   * replace "*/" comment end tokens with "* /", which does not
		//     terminate the comment
		//   * replace "<script" and "</script" with "\x3Cscript" and "\x3C/script"
		//     (case insensitively), and "<!--" with "\x3C!--", which prevents
		//     confusing script block termination semantics
		//
		// We also put a space before the comment so that if it is flush against
		// a division operator it is not turned into a line comment:
		//     x/{{y}}
		// turning into
		//     x//* error marshaling y:
		//          second line of error message */null
		errStr := err.Error()
		errStr = string(scriptTagRe.ReplaceAll([]byte(errStr), []byte(`\x3C${1}script`)))
		errStr = strings.ReplaceAll(errStr, "*/", "* /")
		errStr = strings.ReplaceAll(errStr, "<!--", `\x3C!--`)
		return fmt.Sprintf(" /* %s */null ", errStr)
	}

	// TODO: maybe post-process output to prevent it from containing
	// "<!--", "-->", "<![CDATA[", "]]>", or "</script"
	// in case custom marshalers produce output containing those.
	// Note: Do not use \x escaping to save bytes because it is not JSON compatible and this escaper
	// supports ld+json content-type.
	if len(b) == 0 {
		// In, `x=y/{{.}}*z` a json.Marshaler that produces "" should
		// not cause the output `x=y/*z`.
		return " null "
	}
	first, _ := utf8.DecodeRune(b)
	last, _ := utf8.DecodeLastRune(b)
	var buf strings.Builder
	// Prevent IdentifierNames and NumericLiterals from running into
	// keywords: in, instanceof, typeof, void
	pad := isJSIdentPart(first) || isJSIdentPart(last)
	if pad {
		buf.WriteByte(' ')
	}
	written := 0
	// Make sure that json.Marshal escapes codepoints U+2028 & U+2029
	// so it falls within the subset of JSON which is valid JS.
	for i := 0; i < len(b); {
		rune, n := utf8.DecodeRune(b[i:])
		repl := ""
		if rune == 0x2028 {
			repl = `\u2028`
		} else if rune == 0x2029 {
			repl = `\u2029`
		}
		if repl != "" {
			buf.Write(b[written:i])
			buf.WriteString(repl)
			written = i + n
		}
		i += n
	}
	if buf.Len() != 0 {
		buf.Write(b[written:])
		if pad {
			buf.WriteByte(' ')
		}
		return buf.String()
	}
	return string(b)
}

// jsStrEscaper produces a string that can be included between quotes in
// JavaScript source, in JavaScript embedded in an HTML5 <script> element,
// or in an HTML5 event handler attribute such as onclick.
func jsStrEscaper(args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeJSStr {
		return replace(s, jsStrNormReplacementTable)
	}
	return replace(s, jsStrReplacementTable)
}

func jsTmplLitEscaper(args ...any) string {
	s, _ := stringify(args...)
	return replace(s, jsBqStrReplacementTable)
}

// jsRegexpEscaper behaves like jsStrEscaper but escapes regular expression
// specials so the result is treated literally when included in a regular
// expression literal. /foo{{.X}}bar/ matches the string "foo" followed by
// the literal text of {{.X}} followed by the string "bar".
func jsRegexpEscaper(args ...any) string {
	s, _ := stringify(args...)
	s = replace(s, jsRegexpReplacementTable)
	if s == "" {
		// /{{.X}}/ should not produce a line comment when .X == "".
		return "(?:)"
	}
	return s
}

// replace replaces each rune r of s with replacementTable[r], provided that
// r < len(replacementTable). If replacementTable[r] is the empty string then
// no replacement is made.
// It also replaces runes U+2028 and U+2029 with the raw strings `\u2028` and
// `\u2029`.
func replace(s string, replacementTable []string) string {
	var b strings.Builder
	r, w, written := rune(0), 0, 0
	for i := 0; i < len(s); i += w {
		// See comment in htmlEscaper.
		r, w = utf8.DecodeRuneInString(s[i:])
		var repl string
		switch {
		case int(r) < len(lowUnicodeReplacementTable):
			repl = lowUnicodeReplacementTable[r]
		case int(r) < len(replacementTable) && replacementTable[r] != "":
			repl = replacementTable[r]
		case r == '\u2028':
			repl = `\u2028`
		case r == '\u2029':
			repl = `\u2029`
		default:
			continue
		}
		if written == 0 {
			b.Grow(len(s))
		}
		b.WriteString(s[written:i])
		b.WriteString(repl)
		written = i + w
	}
	if written == 0 {
		return s
	}
	b.WriteString(s[written:])
	return b.String()
}

var lowUnicodeReplacementTable = []string{
	0: `\u0000`, 1: `\u0001`, 2: `\u0002`, 3: `\u0003`, 4: `\u0004`, 5: `\u0005`, 6: `\u0006`,
	'\a': `\u0007`,
	'\b': `\u0008`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	0xe:  `\u000e`, 0xf: `\u000f`, 0x10: `\u0010`, 0x11: `\u0011`, 0x12: `\u0012`, 0x13: `\u0013`,
	0x14: `\u0014`, 0x15: `\u0015`, 0x16: `\u0016`, 0x17: `\u0017`, 0x18: `\u0018`, 0x19: `\u0019`,
	0x1a: `\u001a`, 0x1b: `\u001b`, 0x1c: `\u001c`, 0x1d: `\u001d`, 0x1e: `\u001e`, 0x1f: `\u001f`,
}

var jsStrReplacementTable = []string{
	0:    `\u0000`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\u0022`,
	'`':  `\u0060`,
	'&':  `\u0026`,
	'\'': `\u0027`,
	'+':  `\u002b`,
	'/':  `\/`,
	'<':  `\u003c`,
	'>':  `\u003e`,
	'\\': `\\`,
}

// jsBqStrReplacementTable is like jsStrReplacementTable except it also contains
// the special characters for JS template literals: $, {, and }.
var jsBqStrReplacementTable = []string{
	0:    `\u0000`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\u0022`,
	'`':  `\u0060`,
	'&':  `\u0026`,
	'\'': `\u0027`,
	'+':  `\u002b`,
	'/':  `\/`,
	'<':  `\u003c`,
	'>':  `\u003e`,
	'\\': `\\`,
	'$':  `\u0024`,
	'{':  `\u007b`,
	'}':  `\u007d`,
}

// jsStrNormReplacementTable is like jsStrReplacementTable but does not
// overencode existing escapes since this table has no entry for `\`.
var jsStrNormReplacementTable = []string{
	0:    `\u0000`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\u0022`,
	'&':  `\u0026`,
	'\'': `\u0027`,
	'`':  `\u0060`,
	'+':  `\u002b`,
	'/':  `\/`,
	'<':  `\u003c`,
	'>':  `\u003e`,
}
var jsRegexpReplacementTable = []string{
	0:    `\u0000`,
	'\t': `\t`,
	'\n': `\n`,
	'\v': `\u000b`, // "\v" == "v" on IE 6.
	'\f': `\f`,
	'\r': `\r`,
	// Encode HTML specials as hex so the output can be embedded
	// in HTML attributes without further encoding.
	'"':  `\u0022`,
	'$':  `\$`,
	'&':  `\u0026`,
	'\'': `\u0027`,
	'(':  `\(`,
	')':  `\)`,
	'*':  `\*`,
	'+':  `\u002b`,
	'-':  `\-`,
	'.':  `\.`,
	'/':  `\/`,
	'<':  `\u003c`,
	'>':  `\u003e`,
	'?':  `\?`,
	'[':  `\[`,
	'\\': `\\`,
	']':  `\]`,
	'^':  `\^`,
	'{':  `\{`,
	'|':  `\|`,
	'}':  `\}`,
}

// isJSIdentPart reports whether the given rune is a JS identifier part.
// It does not handle all the non-Latin letters, joiners, and combining marks,
// but it does handle every codepoint that can occur in a numeric literal or
// a keyword.
func isJSIdentPart(r rune) bool {
	switch {
	case r == '$':
		return true
	case '0' <= r && r <= '9':
		return true
	case 'A' <= r && r <= 'Z':
		return true
	case r == '_':
		return true
	case 'a' <= r && r <= 'z':
		return true
	}
	return false
}

// isJSType reports whether the given MIME type should be considered JavaScript.
//
// It is used to determine whether a script tag with a type attribute is a javascript container.
func isJSType(mimeType string) bool {
	// per
	//   https://www.w3.org/TR/html5/scripting-1.html#attr-script-type
	//   https://tools.ietf.org/html/rfc7231#section-3.1.1
	//   https://tools.ietf.org/html/rfc4329#section-3
	//   https://www.ietf.org/rfc/rfc4627.txt
	// discard parameters
	mimeType, _, _ = strings.Cut(mimeType, ";")
	mimeType = strings.ToLower(mimeType)
	mimeType = strings.TrimSpace(mimeType)
	switch mimeType {
	case
		"",
		"application/ecmascript",
		"application/javascript",
		"application/json",
		"application/ld+json",
		"application/x-ecmascript",
		"application/x-javascript",
		"module",
		"text/ecmascript",
		"text/javascript",
		"text/javascript1.0",
		"text/javascript1.1",
		"text/javascript1.2",
		"text/javascript1.3",
		"text/javascript1.4",
		"text/javascript1.5",
		"text/jscript",
		"text/livescript",
		"text/x-ecmascript",
		"text/x-javascript":
		return true
	default:
		return false
	}
}

```

// === FILE: references/go/src/html/template/jsctx_string.go ===
```go
// Code generated by "stringer -type jsCtx"; DO NOT EDIT.

package template

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[jsCtxRegexp-0]
	_ = x[jsCtxDivOp-1]
	_ = x[jsCtxUnknown-2]
}

const _jsCtx_name = "jsCtxRegexpjsCtxDivOpjsCtxUnknown"

var _jsCtx_index = [...]uint8{0, 11, 21, 33}

func (i jsCtx) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_jsCtx_index)-1 {
		return "jsCtx(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _jsCtx_name[_jsCtx_index[idx]:_jsCtx_index[idx+1]]
}

```

// === FILE: references/go/src/html/template/state_string.go ===
```go
// Code generated by "stringer -type state"; DO NOT EDIT.

package template

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[stateText-0]
	_ = x[stateTag-1]
	_ = x[stateAttrName-2]
	_ = x[stateAfterName-3]
	_ = x[stateBeforeValue-4]
	_ = x[stateHTMLCmt-5]
	_ = x[stateRCDATA-6]
	_ = x[stateAttr-7]
	_ = x[stateURL-8]
	_ = x[stateSrcset-9]
	_ = x[stateJS-10]
	_ = x[stateJSDqStr-11]
	_ = x[stateJSSqStr-12]
	_ = x[stateJSTmplLit-13]
	_ = x[stateJSRegexp-14]
	_ = x[stateJSBlockCmt-15]
	_ = x[stateJSLineCmt-16]
	_ = x[stateJSHTMLOpenCmt-17]
	_ = x[stateJSHTMLCloseCmt-18]
	_ = x[stateCSS-19]
	_ = x[stateCSSDqStr-20]
	_ = x[stateCSSSqStr-21]
	_ = x[stateCSSDqURL-22]
	_ = x[stateCSSSqURL-23]
	_ = x[stateCSSURL-24]
	_ = x[stateCSSBlockCmt-25]
	_ = x[stateCSSLineCmt-26]
	_ = x[stateError-27]
	_ = x[stateMetaContent-28]
	_ = x[stateMetaContentURL-29]
	_ = x[stateDead-30]
}

const _state_name = "stateTextstateTagstateAttrNamestateAfterNamestateBeforeValuestateHTMLCmtstateRCDATAstateAttrstateURLstateSrcsetstateJSstateJSDqStrstateJSSqStrstateJSTmplLitstateJSRegexpstateJSBlockCmtstateJSLineCmtstateJSHTMLOpenCmtstateJSHTMLCloseCmtstateCSSstateCSSDqStrstateCSSSqStrstateCSSDqURLstateCSSSqURLstateCSSURLstateCSSBlockCmtstateCSSLineCmtstateErrorstateMetaContentstateMetaContentURLstateDead"

var _state_index = [...]uint16{0, 9, 17, 30, 44, 60, 72, 83, 92, 100, 111, 118, 130, 142, 156, 169, 184, 198, 216, 235, 243, 256, 269, 282, 295, 306, 322, 337, 347, 363, 382, 391}

func (i state) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_state_index)-1 {
		return "state(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _state_name[_state_index[idx]:_state_index[idx+1]]
}

```

// === FILE: references/go/src/html/template/template.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"text/template"
	"text/template/parse"
)

// Template is a specialized Template from "text/template" that produces a safe
// HTML document fragment.
type Template struct {
	// Sticky error if escaping fails, or escapeOK if succeeded.
	escapeErr error
	// We could embed the text/template field, but it's safer not to because
	// we need to keep our version of the name space and the underlying
	// template's in sync.
	text *template.Template
	// The underlying template's parse tree, updated to be HTML-safe.
	Tree       *parse.Tree
	*nameSpace // common to all associated templates
}

// escapeOK is a sentinel value used to indicate valid escaping.
var escapeOK = fmt.Errorf("template escaped correctly")

// nameSpace is the data structure shared by all templates in an association.
type nameSpace struct {
	mu      sync.Mutex
	set     map[string]*Template
	escaped bool
	esc     escaper
}

// Templates returns a slice of the templates associated with t, including t
// itself.
func (t *Template) Templates() []*Template {
	ns := t.nameSpace
	ns.mu.Lock()
	defer ns.mu.Unlock()
	// Return a slice so we don't expose the map.
	m := make([]*Template, 0, len(ns.set))
	for _, v := range ns.set {
		m = append(m, v)
	}
	return m
}

// Option sets options for the template. Options are described by
// strings, either a simple string or "key=value". There can be at
// most one equals sign in an option string. If the option string
// is unrecognized or otherwise invalid, Option panics.
//
// Known options:
//
// missingkey: Control the behavior during execution if a map is
// indexed with a key that is not present in the map.
//
//	"missingkey=default" or "missingkey=invalid"
//		The default behavior: Do nothing and continue execution.
//		If printed, the result of the index operation is the string
//		"<no value>".
//	"missingkey=zero"
//		The operation returns the zero value for the map type's element.
//	"missingkey=error"
//		Execution stops immediately with an error.
func (t *Template) Option(opt ...string) *Template {
	t.text.Option(opt...)
	return t
}

// checkCanParse checks whether it is OK to parse templates.
// If not, it returns an error.
func (t *Template) checkCanParse() error {
	if t == nil {
		return nil
	}
	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	if t.nameSpace.escaped {
		return fmt.Errorf("html/template: cannot Parse after Execute")
	}
	return nil
}

// escape escapes all associated templates.
func (t *Template) escape() error {
	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	t.nameSpace.escaped = true
	if t.escapeErr == nil {
		if t.Tree == nil {
			return fmt.Errorf("template: %q is an incomplete or empty template", t.Name())
		}
		if err := escapeTemplate(t, t.text.Root, t.Name()); err != nil {
			return err
		}
	} else if t.escapeErr != escapeOK {
		return t.escapeErr
	}
	return nil
}

// Execute applies a parsed template to the specified data object,
// writing the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
func (t *Template) Execute(wr io.Writer, data any) error {
	if err := t.escape(); err != nil {
		return err
	}
	return t.text.Execute(wr, data)
}

// ExecuteTemplate applies the template associated with t that has the given
// name to the specified data object and writes the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
func (t *Template) ExecuteTemplate(wr io.Writer, name string, data any) error {
	tmpl, err := t.lookupAndEscapeTemplate(name)
	if err != nil {
		return err
	}
	return tmpl.text.Execute(wr, data)
}

// lookupAndEscapeTemplate guarantees that the template with the given name
// is escaped, or returns an error if it cannot be. It returns the named
// template.
func (t *Template) lookupAndEscapeTemplate(name string) (tmpl *Template, err error) {
	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	t.nameSpace.escaped = true
	tmpl = t.set[name]
	if tmpl == nil {
		return nil, fmt.Errorf("html/template: %q is undefined", name)
	}
	if tmpl.escapeErr != nil && tmpl.escapeErr != escapeOK {
		return nil, tmpl.escapeErr
	}
	if tmpl.text.Tree == nil || tmpl.text.Root == nil {
		return nil, fmt.Errorf("html/template: %q is an incomplete template", name)
	}
	if t.text.Lookup(name) == nil {
		panic("html/template internal error: template escaping out of sync")
	}
	if tmpl.escapeErr == nil {
		err = escapeTemplate(tmpl, tmpl.text.Root, name)
	}
	return tmpl, err
}

// DefinedTemplates returns a string listing the defined templates,
// prefixed by the string "; defined templates are: ". If there are none,
// it returns the empty string. Used to generate an error message.
func (t *Template) DefinedTemplates() string {
	return t.text.DefinedTemplates()
}

// Parse parses text as a template body for t.
// Named template definitions ({{define ...}} or {{block ...}} statements) in text
// define additional templates associated with t and are removed from the
// definition of t itself.
//
// Templates can be redefined in successive calls to Parse,
// before the first use of [Template.Execute] on t or any associated template.
// A template definition with a body containing only white space and comments
// is considered empty and will not replace an existing template's body.
// This allows using Parse to add new named template definitions without
// overwriting the main template body.
func (t *Template) Parse(text string) (*Template, error) {
	if err := t.checkCanParse(); err != nil {
		return nil, err
	}

	ret, err := t.text.Parse(text)
	if err != nil {
		return nil, err
	}

	// In general, all the named templates might have changed underfoot.
	// Regardless, some new ones may have been defined.
	// The template.Template set has been updated; update ours.
	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	for _, v := range ret.Templates() {
		name := v.Name()
		tmpl := t.set[name]
		if tmpl == nil {
			tmpl = t.new(name)
		}
		tmpl.text = v
		tmpl.Tree = v.Tree
	}
	return t, nil
}

// AddParseTree creates a new template with the name and parse tree
// and associates it with t.
//
// It returns an error if t or any associated template has already been executed.
func (t *Template) AddParseTree(name string, tree *parse.Tree) (*Template, error) {
	if err := t.checkCanParse(); err != nil {
		return nil, err
	}

	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	text, err := t.text.AddParseTree(name, tree)
	if err != nil {
		return nil, err
	}
	ret := &Template{
		nil,
		text,
		text.Tree,
		t.nameSpace,
	}
	t.set[name] = ret
	return ret, nil
}

// Clone returns a duplicate of the template, including all associated
// templates. The actual representation is not copied, but the name space of
// associated templates is, so further calls to [Template.Parse] in the copy will add
// templates to the copy but not to the original. [Template.Clone] can be used to prepare
// common templates and use them with variant definitions for other templates
// by adding the variants after the clone is made.
//
// It returns an error if t has already been executed.
func (t *Template) Clone() (*Template, error) {
	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	if t.escapeErr != nil {
		return nil, fmt.Errorf("html/template: cannot Clone %q after it has executed", t.Name())
	}
	textClone, err := t.text.Clone()
	if err != nil {
		return nil, err
	}
	ns := &nameSpace{set: make(map[string]*Template)}
	ns.esc = makeEscaper(ns)
	ret := &Template{
		nil,
		textClone,
		textClone.Tree,
		ns,
	}
	ret.set[ret.Name()] = ret
	for _, x := range textClone.Templates() {
		name := x.Name()
		src := t.set[name]
		if src == nil || src.escapeErr != nil {
			return nil, fmt.Errorf("html/template: cannot Clone %q after it has executed", t.Name())
		}
		x.Tree = x.Tree.Copy()
		ret.set[name] = &Template{
			nil,
			x,
			x.Tree,
			ret.nameSpace,
		}
	}
	// Return the template associated with the name of this template.
	return ret.set[ret.Name()], nil
}

// New allocates a new HTML template with the given name.
func New(name string) *Template {
	ns := &nameSpace{set: make(map[string]*Template)}
	ns.esc = makeEscaper(ns)
	tmpl := &Template{
		nil,
		template.New(name),
		nil,
		ns,
	}
	tmpl.set[name] = tmpl
	return tmpl
}

// New allocates a new HTML template associated with the given one
// and with the same delimiters. The association, which is transitive,
// allows one template to invoke another with a {{template}} action.
//
// If a template with the given name already exists, the new HTML template
// will replace it. The existing template will be reset and disassociated with
// t.
func (t *Template) New(name string) *Template {
	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	return t.new(name)
}

// new is the implementation of New, without the lock.
func (t *Template) new(name string) *Template {
	tmpl := &Template{
		nil,
		t.text.New(name),
		nil,
		t.nameSpace,
	}
	if existing, ok := tmpl.set[name]; ok {
		emptyTmpl := New(existing.Name())
		*existing = *emptyTmpl
	}
	tmpl.set[name] = tmpl
	return tmpl
}

// Name returns the name of the template.
func (t *Template) Name() string {
	return t.text.Name()
}

type FuncMap = template.FuncMap

// Funcs adds the elements of the argument map to the template's function map.
// It must be called before the template is parsed.
// It panics if a value in the map is not a function with appropriate return
// type. However, it is legal to overwrite elements of the map. The return
// value is the template, so calls can be chained.
func (t *Template) Funcs(funcMap FuncMap) *Template {
	t.text.Funcs(template.FuncMap(funcMap))
	return t
}

// Delims sets the action delimiters to the specified strings, to be used in
// subsequent calls to [Template.Parse], [ParseFiles], or [ParseGlob]. Nested template
// definitions will inherit the settings. An empty delimiter stands for the
// corresponding default: {{ or }}.
// The return value is the template, so calls can be chained.
func (t *Template) Delims(left, right string) *Template {
	t.text.Delims(left, right)
	return t
}

// Lookup returns the template with the given name that is associated with t,
// or nil if there is no such template.
func (t *Template) Lookup(name string) *Template {
	t.nameSpace.mu.Lock()
	defer t.nameSpace.mu.Unlock()
	return t.set[name]
}

// Must is a helper that wraps a call to a function returning ([*Template], error)
// and panics if the error is non-nil. It is intended for use in variable initializations
// such as
//
//	var t = template.Must(template.New("name").Parse("html"))
func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return t
}

// ParseFiles creates a new [Template] and parses the template definitions from
// the named files. The returned template's name will have the (base) name and
// (parsed) contents of the first file. There must be at least one file.
// If an error occurs, parsing stops and the returned [*Template] is nil.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
// For instance, ParseFiles("a/foo", "b/foo") stores "b/foo" as the template
// named "foo", while "a/foo" is unavailable.
func ParseFiles(filenames ...string) (*Template, error) {
	return parseFiles(nil, readFileOS, filenames...)
}

// ParseFiles parses the named files and associates the resulting templates with
// t. If an error occurs, parsing stops and the returned template is nil;
// otherwise it is t. There must be at least one file.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
//
// ParseFiles returns an error if t or any associated template has already been executed.
func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	return parseFiles(t, readFileOS, filenames...)
}

// parseFiles is the helper for the method and function. If the argument
// template is nil, it is created from the first file.
func parseFiles(t *Template, readFile func(string) (string, []byte, error), filenames ...string) (*Template, error) {
	if err := t.checkCanParse(); err != nil {
		return nil, err
	}

	if len(filenames) == 0 {
		// Not really a problem, but be consistent.
		return nil, fmt.Errorf("html/template: no files named in call to ParseFiles")
	}
	for _, filename := range filenames {
		name, b, err := readFile(filename)
		if err != nil {
			return nil, err
		}
		s := string(b)
		// First template becomes return value if not already defined,
		// and we use that one for subsequent New calls to associate
		// all the templates together. Also, if this file has the same name
		// as t, this file becomes the contents of t, so
		//  t, err := New(name).Funcs(xxx).ParseFiles(name)
		// works. Otherwise we create a new template associated with t.
		var tmpl *Template
		if t == nil {
			t = New(name)
		}
		if name == t.Name() {
			tmpl = t
		} else {
			tmpl = t.New(name)
		}
		_, err = tmpl.Parse(s)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

// ParseGlob creates a new [Template] and parses the template definitions from
// the files identified by the pattern. The files are matched according to the
// semantics of filepath.Match, and the pattern must match at least one file.
// The returned template will have the (base) name and (parsed) contents of the
// first file matched by the pattern. ParseGlob is equivalent to calling
// [ParseFiles] with the list of files matched by the pattern.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
func ParseGlob(pattern string) (*Template, error) {
	return parseGlob(nil, pattern)
}

// ParseGlob parses the template definitions in the files identified by the
// pattern and associates the resulting templates with t. The files are matched
// according to the semantics of filepath.Match, and the pattern must match at
// least one file. ParseGlob is equivalent to calling t.ParseFiles with the
// list of files matched by the pattern.
//
// When parsing multiple files with the same name in different directories,
// the last one mentioned will be the one that results.
//
// ParseGlob returns an error if t or any associated template has already been executed.
func (t *Template) ParseGlob(pattern string) (*Template, error) {
	return parseGlob(t, pattern)
}

// parseGlob is the implementation of the function and method ParseGlob.
func parseGlob(t *Template, pattern string) (*Template, error) {
	if err := t.checkCanParse(); err != nil {
		return nil, err
	}
	filenames, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(filenames) == 0 {
		return nil, fmt.Errorf("html/template: pattern matches no files: %#q", pattern)
	}
	return parseFiles(t, readFileOS, filenames...)
}

// IsTrue reports whether the value is 'true', in the sense of not the zero of its type,
// and whether the value has a meaningful truth value. This is the definition of
// truth used by if and other such actions.
func IsTrue(val any) (truth, ok bool) {
	return template.IsTrue(val)
}

// ParseFS is like [ParseFiles] or [ParseGlob] but reads from the file system fs
// instead of the host operating system's file system.
// It accepts a list of glob patterns.
// (Note that most file names serve as glob patterns matching only themselves.)
func ParseFS(fs fs.FS, patterns ...string) (*Template, error) {
	return parseFS(nil, fs, patterns)
}

// ParseFS is like [Template.ParseFiles] or [Template.ParseGlob] but reads from the file system fs
// instead of the host operating system's file system.
// It accepts a list of glob patterns.
// (Note that most file names serve as glob patterns matching only themselves.)
func (t *Template) ParseFS(fs fs.FS, patterns ...string) (*Template, error) {
	return parseFS(t, fs, patterns)
}

func parseFS(t *Template, fsys fs.FS, patterns []string) (*Template, error) {
	var filenames []string
	for _, pattern := range patterns {
		list, err := fs.Glob(fsys, pattern)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("template: pattern matches no files: %#q", pattern)
		}
		filenames = append(filenames, list...)
	}
	return parseFiles(t, readFileFS(fsys), filenames...)
}

func readFileOS(file string) (name string, b []byte, err error) {
	name = filepath.Base(file)
	b, err = os.ReadFile(file)
	return
}

func readFileFS(fsys fs.FS) func(string) (string, []byte, error) {
	return func(file string) (name string, b []byte, err error) {
		name = path.Base(file)
		b, err = fs.ReadFile(fsys, file)
		return
	}
}

```

// === FILE: references/go/src/html/template/transition.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"bytes"
	"strings"
)

// transitionFunc is the array of context transition functions for text nodes.
// A transition function takes a context and template text input, and returns
// the updated context and the number of bytes consumed from the front of the
// input.
var transitionFunc = [...]func(context, []byte) (context, int){
	stateText:           tText,
	stateTag:            tTag,
	stateAttrName:       tAttrName,
	stateAfterName:      tAfterName,
	stateBeforeValue:    tBeforeValue,
	stateHTMLCmt:        tHTMLCmt,
	stateRCDATA:         tSpecialTagEnd,
	stateAttr:           tAttr,
	stateURL:            tURL,
	stateMetaContent:    tMetaContent,
	stateMetaContentURL: tMetaContentURL,
	stateSrcset:         tURL,
	stateJS:             tJS,
	stateJSDqStr:        tJSDelimited,
	stateJSSqStr:        tJSDelimited,
	stateJSRegexp:       tJSDelimited,
	stateJSTmplLit:      tJSTmpl,
	stateJSBlockCmt:     tBlockCmt,
	stateJSLineCmt:      tLineCmt,
	stateJSHTMLOpenCmt:  tLineCmt,
	stateJSHTMLCloseCmt: tLineCmt,
	stateCSS:            tCSS,
	stateCSSDqStr:       tCSSStr,
	stateCSSSqStr:       tCSSStr,
	stateCSSDqURL:       tCSSStr,
	stateCSSSqURL:       tCSSStr,
	stateCSSURL:         tCSSStr,
	stateCSSBlockCmt:    tBlockCmt,
	stateCSSLineCmt:     tLineCmt,
	stateError:          tError,
}

var commentStart = []byte("<!--")
var commentEnd = []byte("-->")

// tText is the context transition function for the text state.
func tText(c context, s []byte) (context, int) {
	k := 0
	for {
		i := k + bytes.IndexByte(s[k:], '<')
		if i < k || i+1 == len(s) {
			return c, len(s)
		} else if i+4 <= len(s) && bytes.Equal(commentStart, s[i:i+4]) {
			return context{state: stateHTMLCmt}, i + 4
		}
		i++
		end := false
		if s[i] == '/' {
			if i+1 == len(s) {
				return c, len(s)
			}
			end, i = true, i+1
		}
		j, e := eatTagName(s, i)
		if j != i {
			if end {
				e = elementNone
			}
			// We've found an HTML tag.
			return context{state: stateTag, element: e}, j
		}
		k = j
	}
}

var elementContentType = [...]state{
	elementNone:     stateText,
	elementScript:   stateJS,
	elementStyle:    stateCSS,
	elementTextarea: stateRCDATA,
	elementTitle:    stateRCDATA,
	elementMeta:     stateText,
}

// tTag is the context transition function for the tag state.
func tTag(c context, s []byte) (context, int) {
	// Find the attribute name.
	i := eatWhiteSpace(s, 0)
	if i == len(s) {
		return c, len(s)
	}
	if s[i] == '>' {
		// Treat <meta> specially, because it doesn't have an end tag, and we
		// want to transition into the correct state/element for it.
		if c.element == elementMeta {
			return context{state: stateText, element: elementNone}, i + 1
		}
		return context{
			state:   elementContentType[c.element],
			element: c.element,
		}, i + 1
	}
	j, err := eatAttrName(s, i)
	if err != nil {
		return context{state: stateError, err: err}, len(s)
	}
	state, attr := stateTag, attrNone
	if i == j {
		return context{
			state: stateError,
			err:   errorf(ErrBadHTML, nil, 0, "expected space, attr name, or end of tag, but got %q", s[i:]),
		}, len(s)
	}

	attrName := strings.ToLower(string(s[i:j]))
	if c.element == elementScript && attrName == "type" {
		attr = attrScriptType
	} else if c.element == elementMeta && attrName == "content" {
		attr = attrMetaContent
	} else {
		switch attrType(attrName) {
		case contentTypeURL:
			attr = attrURL
		case contentTypeCSS:
			attr = attrStyle
		case contentTypeJS:
			attr = attrScript
		case contentTypeSrcset:
			attr = attrSrcset
		}
	}

	if j == len(s) {
		state = stateAttrName
	} else {
		state = stateAfterName
	}
	return context{state: state, element: c.element, attr: attr}, j
}

// tAttrName is the context transition function for stateAttrName.
func tAttrName(c context, s []byte) (context, int) {
	i, err := eatAttrName(s, 0)
	if err != nil {
		return context{state: stateError, err: err}, len(s)
	} else if i != len(s) {
		c.state = stateAfterName
	}
	return c, i
}

// tAfterName is the context transition function for stateAfterName.
func tAfterName(c context, s []byte) (context, int) {
	// Look for the start of the value.
	i := eatWhiteSpace(s, 0)
	if i == len(s) {
		return c, len(s)
	} else if s[i] != '=' {
		// Occurs due to tag ending '>', and valueless attribute.
		c.state = stateTag
		return c, i
	}
	c.state = stateBeforeValue
	// Consume the "=".
	return c, i + 1
}

var attrStartStates = [...]state{
	attrNone:        stateAttr,
	attrScript:      stateJS,
	attrScriptType:  stateAttr,
	attrStyle:       stateCSS,
	attrURL:         stateURL,
	attrSrcset:      stateSrcset,
	attrMetaContent: stateMetaContent,
}

// tBeforeValue is the context transition function for stateBeforeValue.
func tBeforeValue(c context, s []byte) (context, int) {
	i := eatWhiteSpace(s, 0)
	if i == len(s) {
		return c, len(s)
	}
	// Find the attribute delimiter.
	delim := delimSpaceOrTagEnd
	switch s[i] {
	case '\'':
		delim, i = delimSingleQuote, i+1
	case '"':
		delim, i = delimDoubleQuote, i+1
	}
	c.state, c.delim = attrStartStates[c.attr], delim
	return c, i
}

// tHTMLCmt is the context transition function for stateHTMLCmt.
func tHTMLCmt(c context, s []byte) (context, int) {
	if i := bytes.Index(s, commentEnd); i != -1 {
		return context{}, i + 3
	}
	return c, len(s)
}

// specialTagEndMarkers maps element types to the character sequence that
// case-insensitively signals the end of the special tag body.
var specialTagEndMarkers = [...][]byte{
	elementScript:   []byte("script"),
	elementStyle:    []byte("style"),
	elementTextarea: []byte("textarea"),
	elementTitle:    []byte("title"),
	elementMeta:     []byte(""),
}

var (
	specialTagEndPrefix = []byte("</")
	tagEndSeparators    = []byte("> \t\n\f/")
)

// tSpecialTagEnd is the context transition function for raw text and RCDATA
// element states.
func tSpecialTagEnd(c context, s []byte) (context, int) {
	if c.element != elementNone {
		// script end tags ("</script") within script literals are ignored, so that
		// we can properly escape them.
		if c.element == elementScript && (isInScriptLiteral(c.state) || isComment(c.state)) {
			return c, len(s)
		}
		if i := indexTagEnd(s, specialTagEndMarkers[c.element]); i != -1 {
			return context{}, i
		}
	}
	return c, len(s)
}

// indexTagEnd finds the index of a special tag end in a case insensitive way, or returns -1
func indexTagEnd(s []byte, tag []byte) int {
	res := 0
	plen := len(specialTagEndPrefix)
	for len(s) > 0 {
		// Try to find the tag end prefix first
		i := bytes.Index(s, specialTagEndPrefix)
		if i == -1 {
			return i
		}
		s = s[i+plen:]
		// Try to match the actual tag if there is still space for it
		if len(tag) <= len(s) && bytes.EqualFold(tag, s[:len(tag)]) {
			s = s[len(tag):]
			// Check the tag is followed by a proper separator
			if len(s) > 0 && bytes.IndexByte(tagEndSeparators, s[0]) != -1 {
				return res + i
			}
			res += len(tag)
		}
		res += i + plen
	}
	return -1
}

// tAttr is the context transition function for the attribute state.
func tAttr(c context, s []byte) (context, int) {
	return c, len(s)
}

// tURL is the context transition function for the URL state.
func tURL(c context, s []byte) (context, int) {
	if bytes.ContainsAny(s, "#?") {
		c.urlPart = urlPartQueryOrFrag
	} else if len(s) != eatWhiteSpace(s, 0) && c.urlPart == urlPartNone {
		// HTML5 uses "Valid URL potentially surrounded by spaces" for
		// attrs: https://www.w3.org/TR/html5/index.html#attributes-1
		c.urlPart = urlPartPreQuery
	}
	return c, len(s)
}

// tJS is the context transition function for the JS state.
func tJS(c context, s []byte) (context, int) {
	i := bytes.IndexAny(s, "\"`'/{}<-#")
	if i == -1 {
		// Entire input is non string, comment, regexp tokens.
		c.jsCtx = nextJSCtx(s, c.jsCtx)
		return c, len(s)
	}
	c.jsCtx = nextJSCtx(s[:i], c.jsCtx)
	switch s[i] {
	case '"':
		c.state, c.jsCtx = stateJSDqStr, jsCtxRegexp
	case '\'':
		c.state, c.jsCtx = stateJSSqStr, jsCtxRegexp
	case '`':
		c.state, c.jsCtx = stateJSTmplLit, jsCtxRegexp
	case '/':
		switch {
		case i+1 < len(s) && s[i+1] == '/':
			c.state, i = stateJSLineCmt, i+1
		case i+1 < len(s) && s[i+1] == '*':
			c.state, i = stateJSBlockCmt, i+1
		case c.jsCtx == jsCtxRegexp:
			c.state = stateJSRegexp
		case c.jsCtx == jsCtxDivOp:
			c.jsCtx = jsCtxRegexp
		default:
			return context{
				state: stateError,
				err:   errorf(ErrSlashAmbig, nil, 0, "'/' could start a division or regexp: %.32q", s[i:]),
			}, len(s)
		}
	// ECMAScript supports HTML style comments for legacy reasons, see Appendix
	// B.1.1 "HTML-like Comments". The handling of these comments is somewhat
	// confusing. Multi-line comments are not supported, i.e. anything on lines
	// between the opening and closing tokens is not considered a comment, but
	// anything following the opening or closing token, on the same line, is
	// ignored. As such we simply treat any line prefixed with "<!--" or "-->"
	// as if it were actually prefixed with "//" and move on.
	case '<':
		if i+3 < len(s) && bytes.Equal(commentStart, s[i:i+4]) {
			c.state, i = stateJSHTMLOpenCmt, i+3
		}
	case '-':
		if i+2 < len(s) && bytes.Equal(commentEnd, s[i:i+3]) {
			c.state, i = stateJSHTMLCloseCmt, i+2
		}
	// ECMAScript also supports "hashbang" comment lines, see Section 12.5.
	case '#':
		if i+1 < len(s) && s[i+1] == '!' {
			c.state, i = stateJSLineCmt, i+1
		}
	case '{':
		// We only care about tracking brace depth if we are inside of a
		// template literal.
		if len(c.jsBraceDepth) == 0 {
			return c, i + 1
		}
		c.jsBraceDepth[len(c.jsBraceDepth)-1]++
	case '}':
		if len(c.jsBraceDepth) == 0 {
			return c, i + 1
		}
		// There are no cases where a brace can be escaped in the JS context
		// that are not syntax errors, it seems. Because of this we can just
		// count "\}" as "}" and move on, the script is already broken as
		// fully fledged parsers will just fail anyway.
		c.jsBraceDepth[len(c.jsBraceDepth)-1]--
		if c.jsBraceDepth[len(c.jsBraceDepth)-1] >= 0 {
			return c, i + 1
		}
		c.jsBraceDepth = c.jsBraceDepth[:len(c.jsBraceDepth)-1]
		c.state = stateJSTmplLit
	default:
		panic("unreachable")
	}
	return c, i + 1
}

func tJSTmpl(c context, s []byte) (context, int) {
	var k int
	for {
		i := k + bytes.IndexAny(s[k:], "`\\$")
		if i < k {
			break
		}
		switch s[i] {
		case '\\':
			i++
			if i == len(s) {
				return context{
					state: stateError,
					err:   errorf(ErrPartialEscape, nil, 0, "unfinished escape sequence in JS string: %q", s),
				}, len(s)
			}
		case '$':
			if len(s) >= i+2 && s[i+1] == '{' {
				c.jsBraceDepth = append(c.jsBraceDepth, 0)
				c.state = stateJS
				return c, i + 2
			}
		case '`':
			// end
			c.state = stateJS
			return c, i + 1
		}
		k = i + 1
	}

	return c, len(s)
}

// tJSDelimited is the context transition function for the JS string and regexp
// states.
func tJSDelimited(c context, s []byte) (context, int) {
	specials := `\"`
	switch c.state {
	case stateJSSqStr:
		specials = `\'`
	case stateJSRegexp:
		specials = `\/[]`
	}

	k, inCharset := 0, false
	for {
		i := k + bytes.IndexAny(s[k:], specials)
		if i < k {
			break
		}
		switch s[i] {
		case '\\':
			i++
			if i == len(s) {
				return context{
					state: stateError,
					err:   errorf(ErrPartialEscape, nil, 0, "unfinished escape sequence in JS string: %q", s),
				}, len(s)
			}
		case '[':
			inCharset = true
		case ']':
			inCharset = false
		case '/':
			// If "</script" appears in a regex literal, the '/' should not
			// close the regex literal, and it will later be escaped to
			// "\x3C/script" in escapeText.
			if i > 0 && i+7 <= len(s) && bytes.EqualFold(s[i-1:i+7], []byte("</script")) {
				i++
			} else if !inCharset {
				c.state, c.jsCtx = stateJS, jsCtxDivOp
				return c, i + 1
			}
		default:
			// end delimiter
			if !inCharset {
				c.state, c.jsCtx = stateJS, jsCtxDivOp
				return c, i + 1
			}
		}
		k = i + 1
	}

	if inCharset {
		// This can be fixed by making context richer if interpolation
		// into charsets is desired.
		return context{
			state: stateError,
			err:   errorf(ErrPartialCharset, nil, 0, "unfinished JS regexp charset: %q", s),
		}, len(s)
	}

	return c, len(s)
}

var blockCommentEnd = []byte("*/")

// tBlockCmt is the context transition function for /*comment*/ states.
func tBlockCmt(c context, s []byte) (context, int) {
	i := bytes.Index(s, blockCommentEnd)
	if i == -1 {
		return c, len(s)
	}
	switch c.state {
	case stateJSBlockCmt:
		c.state = stateJS
	case stateCSSBlockCmt:
		c.state = stateCSS
	default:
		panic(c.state.String())
	}
	return c, i + 2
}

// tLineCmt is the context transition function for //comment states, and the JS HTML-like comment state.
func tLineCmt(c context, s []byte) (context, int) {
	var lineTerminators string
	var endState state
	switch c.state {
	case stateJSLineCmt, stateJSHTMLOpenCmt, stateJSHTMLCloseCmt:
		lineTerminators, endState = "\n\r\u2028\u2029", stateJS
	case stateCSSLineCmt:
		lineTerminators, endState = "\n\f\r", stateCSS
		// Line comments are not part of any published CSS standard but
		// are supported by the 4 major browsers.
		// This defines line comments as
		//     LINECOMMENT ::= "//" [^\n\f\d]*
		// since https://www.w3.org/TR/css3-syntax/#SUBTOK-nl defines
		// newlines:
		//     nl ::= #xA | #xD #xA | #xD | #xC
	default:
		panic(c.state.String())
	}

	i := bytes.IndexAny(s, lineTerminators)
	if i == -1 {
		return c, len(s)
	}
	c.state = endState
	// Per section 7.4 of EcmaScript 5 : https://es5.github.io/#x7.4
	// "However, the LineTerminator at the end of the line is not
	// considered to be part of the single-line comment; it is
	// recognized separately by the lexical grammar and becomes part
	// of the stream of input elements for the syntactic grammar."
	return c, i
}

// tCSS is the context transition function for the CSS state.
func tCSS(c context, s []byte) (context, int) {
	// CSS quoted strings are almost never used except for:
	// (1) URLs as in background: "/foo.png"
	// (2) Multiword font-names as in font-family: "Times New Roman"
	// (3) List separators in content values as in inline-lists:
	//    <style>
	//    ul.inlineList { list-style: none; padding:0 }
	//    ul.inlineList > li { display: inline }
	//    ul.inlineList > li:before { content: ", " }
	//    ul.inlineList > li:first-child:before { content: "" }
	//    </style>
	//    <ul class=inlineList><li>One<li>Two<li>Three</ul>
	// (4) Attribute value selectors as in a[href="http://example.com/"]
	//
	// We conservatively treat all strings as URLs, but make some
	// allowances to avoid confusion.
	//
	// In (1), our conservative assumption is justified.
	// In (2), valid font names do not contain ':', '?', or '#', so our
	// conservative assumption is fine since we will never transition past
	// urlPartPreQuery.
	// In (3), our protocol heuristic should not be tripped, and there
	// should not be non-space content after a '?' or '#', so as long as
	// we only %-encode RFC 3986 reserved characters we are ok.
	// In (4), we should URL escape for URL attributes, and for others we
	// have the attribute name available if our conservative assumption
	// proves problematic for real code.

	k := 0
	for {
		i := k + bytes.IndexAny(s[k:], `("'/`)
		if i < k {
			return c, len(s)
		}
		switch s[i] {
		case '(':
			// Look for url to the left.
			p := bytes.TrimRight(s[:i], "\t\n\f\r ")
			if endsWithCSSKeyword(p, "url") {
				j := len(s) - len(bytes.TrimLeft(s[i+1:], "\t\n\f\r "))
				switch {
				case j != len(s) && s[j] == '"':
					c.state, j = stateCSSDqURL, j+1
				case j != len(s) && s[j] == '\'':
					c.state, j = stateCSSSqURL, j+1
				default:
					c.state = stateCSSURL
				}
				return c, j
			}
		case '/':
			if i+1 < len(s) {
				switch s[i+1] {
				case '/':
					c.state = stateCSSLineCmt
					return c, i + 2
				case '*':
					c.state = stateCSSBlockCmt
					return c, i + 2
				}
			}
		case '"':
			c.state = stateCSSDqStr
			return c, i + 1
		case '\'':
			c.state = stateCSSSqStr
			return c, i + 1
		}
		k = i + 1
	}
}

// tCSSStr is the context transition function for the CSS string and URL states.
func tCSSStr(c context, s []byte) (context, int) {
	var endAndEsc string
	switch c.state {
	case stateCSSDqStr, stateCSSDqURL:
		endAndEsc = `\"`
	case stateCSSSqStr, stateCSSSqURL:
		endAndEsc = `\'`
	case stateCSSURL:
		// Unquoted URLs end with a newline or close parenthesis.
		// The below includes the wc (whitespace character) and nl.
		endAndEsc = "\\\t\n\f\r )"
	default:
		panic(c.state.String())
	}

	k := 0
	for {
		i := k + bytes.IndexAny(s[k:], endAndEsc)
		if i < k {
			c, nread := tURL(c, decodeCSS(s[k:]))
			return c, k + nread
		}
		if s[i] == '\\' {
			i++
			if i == len(s) {
				return context{
					state: stateError,
					err:   errorf(ErrPartialEscape, nil, 0, "unfinished escape sequence in CSS string: %q", s),
				}, len(s)
			}
		} else {
			c.state = stateCSS
			return c, i + 1
		}
		c, _ = tURL(c, decodeCSS(s[:i+1]))
		k = i + 1
	}
}

// tError is the context transition function for the error state.
func tError(c context, s []byte) (context, int) {
	return c, len(s)
}

// tMetaContent is the context transition function for the meta content attribute state.
func tMetaContent(c context, s []byte) (context, int) {
	for i := range len(s) {
		if i+3 <= len(s)-1 && bytes.EqualFold(s[i:i+3], []byte("url")) {
			if j := eatWhiteSpace(s, i+3); j < len(s) && s[j] == '=' {
				c.state = stateMetaContentURL
				return c, j + 1
			}
		}
	}
	return c, len(s)
}

// tMetaContentURL is the context transition function for the "url=" part of a meta content attribute state.
func tMetaContentURL(c context, s []byte) (context, int) {
	for i := range len(s) {
		if s[i] == ';' {
			c.state = stateMetaContent
			return c, i + 1
		}
	}
	return c, len(s)
}

// eatAttrName returns the largest j such that s[i:j] is an attribute name.
// It returns an error if s[i:] does not look like it begins with an
// attribute name, such as encountering a quote mark without a preceding
// equals sign.
func eatAttrName(s []byte, i int) (int, *Error) {
	for j := i; j < len(s); j++ {
		switch s[j] {
		case ' ', '\t', '\n', '\f', '\r', '=', '>':
			return j, nil
		case '\'', '"', '<':
			// These result in a parse warning in HTML5 and are
			// indicative of serious problems if seen in an attr
			// name in a template.
			return -1, errorf(ErrBadHTML, nil, 0, "%q in attribute name: %.32q", s[j:j+1], s)
		default:
			// No-op.
		}
	}
	return len(s), nil
}

var elementNameMap = map[string]element{
	"script":   elementScript,
	"style":    elementStyle,
	"textarea": elementTextarea,
	"title":    elementTitle,
	"meta":     elementMeta,
}

// asciiAlpha reports whether c is an ASCII letter.
func asciiAlpha(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z'
}

// asciiAlphaNum reports whether c is an ASCII letter or digit.
func asciiAlphaNum(c byte) bool {
	return asciiAlpha(c) || '0' <= c && c <= '9'
}

// eatTagName returns the largest j such that s[i:j] is a tag name and the tag type.
func eatTagName(s []byte, i int) (int, element) {
	if i == len(s) || !asciiAlpha(s[i]) {
		return i, elementNone
	}
	j := i + 1
	for j < len(s) {
		x := s[j]
		if asciiAlphaNum(x) {
			j++
			continue
		}
		// Allow "x-y" or "x:y" but not "x-", "-y", or "x--y".
		if (x == ':' || x == '-') && j+1 < len(s) && asciiAlphaNum(s[j+1]) {
			j += 2
			continue
		}
		break
	}
	return j, elementNameMap[strings.ToLower(string(s[i:j]))]
}

// eatWhiteSpace returns the largest j such that s[i:j] is white space.
func eatWhiteSpace(s []byte, i int) int {
	for j := i; j < len(s); j++ {
		switch s[j] {
		case ' ', '\t', '\n', '\f', '\r':
			// No-op.
		default:
			return j
		}
	}
	return len(s)
}

```

// === FILE: references/go/src/html/template/url.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"strings"
)

// urlFilter returns its input unless it contains an unsafe scheme in which
// case it defangs the entire URL.
//
// Schemes that cause unintended side effects that are irreversible without user
// interaction are considered unsafe. For example, clicking on a "javascript:"
// link can immediately trigger JavaScript code execution.
//
// This filter conservatively assumes that all schemes other than the following
// are unsafe:
//   - http:   Navigates to a new website, and may open a new window or tab.
//     These side effects can be reversed by navigating back to the
//     previous website, or closing the window or tab. No irreversible
//     changes will take place without further user interaction with
//     the new website.
//   - https:  Same as http.
//   - mailto: Opens an email program and starts a new draft. This side effect
//     is not irreversible until the user explicitly clicks send; it
//     can be undone by closing the email program.
//
// To allow URLs containing other schemes to bypass this filter, developers must
// explicitly indicate that such a URL is expected and safe by encapsulating it
// in a template.URL value.
func urlFilter(args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeURL {
		return s
	}
	if !isSafeURL(s) {
		return "#" + filterFailsafe
	}
	return s
}

// isSafeURL is true if s is a relative URL or if URL has a protocol in
// (http, https, mailto).
func isSafeURL(s string) bool {
	if protocol, _, ok := strings.Cut(s, ":"); ok && !strings.Contains(protocol, "/") {
		if !strings.EqualFold(protocol, "http") && !strings.EqualFold(protocol, "https") && !strings.EqualFold(protocol, "mailto") {
			return false
		}
	}
	return true
}

// urlEscaper produces an output that can be embedded in a URL query.
// The output can be embedded in an HTML attribute without further escaping.
func urlEscaper(args ...any) string {
	return urlProcessor(false, args...)
}

// urlNormalizer normalizes URL content so it can be embedded in a quote-delimited
// string or parenthesis delimited url(...).
// The normalizer does not encode all HTML specials. Specifically, it does not
// encode '&' so correct embedding in an HTML attribute requires escaping of
// '&' to '&amp;'.
func urlNormalizer(args ...any) string {
	return urlProcessor(true, args...)
}

// urlProcessor normalizes (when norm is true) or escapes its input to produce
// a valid hierarchical or opaque URL part.
func urlProcessor(norm bool, args ...any) string {
	s, t := stringify(args...)
	if t == contentTypeURL {
		norm = true
	}
	var b strings.Builder
	if processURLOnto(s, norm, &b) {
		return b.String()
	}
	return s
}

// processURLOnto appends a normalized URL corresponding to its input to b
// and reports whether the appended content differs from s.
func processURLOnto(s string, norm bool, b *strings.Builder) bool {
	b.Grow(len(s) + 16)
	written := 0
	// The byte loop below assumes that all URLs use UTF-8 as the
	// content-encoding. This is similar to the URI to IRI encoding scheme
	// defined in section 3.1 of  RFC 3987, and behaves the same as the
	// EcmaScript builtin encodeURIComponent.
	// It should not cause any misencoding of URLs in pages with
	// Content-type: text/html;charset=UTF-8.
	for i, n := 0, len(s); i < n; i++ {
		c := s[i]
		switch c {
		// Single quote and parens are sub-delims in RFC 3986, but we
		// escape them so the output can be embedded in single
		// quoted attributes and unquoted CSS url(...) constructs.
		// Single quotes are reserved in URLs, but are only used in
		// the obsolete "mark" rule in an appendix in RFC 3986
		// so can be safely encoded.
		case '!', '#', '$', '&', '*', '+', ',', '/', ':', ';', '=', '?', '@', '[', ']':
			if norm {
				continue
			}
		// Unreserved according to RFC 3986 sec 2.3
		// "For consistency, percent-encoded octets in the ranges of
		// ALPHA (%41-%5A and %61-%7A), DIGIT (%30-%39), hyphen (%2D),
		// period (%2E), underscore (%5F), or tilde (%7E) should not be
		// created by URI producers
		case '-', '.', '_', '~':
			continue
		case '%':
			// When normalizing do not re-encode valid escapes.
			if norm && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
				continue
			}
		default:
			// Unreserved according to RFC 3986 sec 2.3
			if 'a' <= c && c <= 'z' {
				continue
			}
			if 'A' <= c && c <= 'Z' {
				continue
			}
			if '0' <= c && c <= '9' {
				continue
			}
		}
		b.WriteString(s[written:i])
		fmt.Fprintf(b, "%%%02x", c)
		written = i + 1
	}
	b.WriteString(s[written:])
	return written != 0
}

// Filters and normalizes srcset values which are comma separated
// URLs followed by metadata.
func srcsetFilterAndEscaper(args ...any) string {
	s, t := stringify(args...)
	switch t {
	case contentTypeSrcset:
		return s
	case contentTypeURL:
		// Normalizing gets rid of all HTML whitespace
		// which separate the image URL from its metadata.
		var b strings.Builder
		if processURLOnto(s, true, &b) {
			s = b.String()
		}
		// Additionally, commas separate one source from another.
		return strings.ReplaceAll(s, ",", "%2c")
	}

	var b strings.Builder
	written := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			filterSrcsetElement(s, written, i, &b)
			b.WriteString(",")
			written = i + 1
		}
	}
	filterSrcsetElement(s, written, len(s), &b)
	return b.String()
}

// Derived from https://play.golang.org/p/Dhmj7FORT5
const htmlSpaceAndASCIIAlnumBytes = "\x00\x36\x00\x00\x01\x00\xff\x03\xfe\xff\xff\x07\xfe\xff\xff\x07"

// isHTMLSpace is true iff c is a whitespace character per
// https://infra.spec.whatwg.org/#ascii-whitespace
func isHTMLSpace(c byte) bool {
	return (c <= 0x20) && 0 != (htmlSpaceAndASCIIAlnumBytes[c>>3]&(1<<uint(c&0x7)))
}

func isHTMLSpaceOrASCIIAlnum(c byte) bool {
	return (c < 0x80) && 0 != (htmlSpaceAndASCIIAlnumBytes[c>>3]&(1<<uint(c&0x7)))
}

func filterSrcsetElement(s string, left int, right int, b *strings.Builder) {
	start := left
	for start < right && isHTMLSpace(s[start]) {
		start++
	}
	end := right
	for i := start; i < right; i++ {
		if isHTMLSpace(s[i]) {
			end = i
			break
		}
	}
	if url := s[start:end]; isSafeURL(url) {
		// If image metadata is only spaces or alnums then
		// we don't need to URL normalize it.
		metadataOk := true
		for i := end; i < right; i++ {
			if !isHTMLSpaceOrASCIIAlnum(s[i]) {
				metadataOk = false
				break
			}
		}
		if metadataOk {
			b.WriteString(s[left:start])
			processURLOnto(url, true, b)
			b.WriteString(s[end:right])
			return
		}
	}
	b.WriteString("#")
	b.WriteString(filterFailsafe)
}

```

// === FILE: references/go/src/html/template/urlpart_string.go ===
```go
// Code generated by "stringer -type urlPart"; DO NOT EDIT.

package template

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[urlPartNone-0]
	_ = x[urlPartPreQuery-1]
	_ = x[urlPartQueryOrFrag-2]
	_ = x[urlPartUnknown-3]
}

const _urlPart_name = "urlPartNoneurlPartPreQueryurlPartQueryOrFragurlPartUnknown"

var _urlPart_index = [...]uint8{0, 11, 26, 44, 58}

func (i urlPart) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_urlPart_index)-1 {
		return "urlPart(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _urlPart_name[_urlPart_index[idx]:_urlPart_index[idx+1]]
}

```

