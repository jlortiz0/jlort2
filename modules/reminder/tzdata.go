package reminder

var timezones = map[string]string{
	"GMT":       "GMT",
	"GHST":      "Africa/Accra",
	"EAT":       "Africa/Kampala", // East Africa
	"CET":       "CET",            // Central Europe
	"CEST":      "CET",
	"WAT":       "Africa/Lagos",  // West Africa
	"CAT":       "Africa/Lusaka", // Central Africa
	"CAST":      "Africa/Khartoum",
	"EET":       "EET", // Eastern Europe
	"EEST":      "EET",
	"WET":       "WET", // Western Europe
	"WEST":      "WET",
	"SAST":      "Africa/Johannesburg", // South Africa
	"HAT":       "HST",                 // Hawaii
	"HAST":      "HST",
	"HADT":      "US/Hawaii",
	"AKT":       "US/Alaska",
	"AKST":      "US/Alaska",
	"AKDT":      "US/Alaska",
	"AT":        "Canada/Atlantic",
	"AST":       "Canada/Atlantic",
	"BRT":       "Brazil/East",
	"BRST":      "Brazil/East",
	"ART":       "America/Argentina/Buenos_Aires",
	"ARST":      "America/Argentina/Buenos_Aires",
	"PYT":       "America/Asuncion",
	"PYST":      "America/Asuncion",
	"ET":        "EST",
	"EST":       "EST",
	"CT":        "US/Central",
	"CST":       "US/Central",
	"CDT":       "US/Central",
	"ADT":       "Canada/Atlantic",
	"AMT":       "Brazil/West",
	"AMST":      "Brazil/West",
	"COT":       "America/Bogota",
	"COST":      "America/Bogota",
	"MT":        "MST",
	"MST":       "MST",
	"MDT":       "US/Mountain",
	"VET":       "America/Caracas",
	"GFT":       "America/Cayenne",
	"PT":        "US/Pacific",
	"PST":       "US/Pacific",
	"EDT":       "US/Eastern",
	"ACT":       "Brazil/Acre",
	"ACST":      "Brazil/Acre",
	"PDT":       "US/Pacific",
	"WGT":       "America/Godthab",
	"WGST":      "America/Godthab",
	"ECT":       "America/Guayaquil",
	"GYT":       "America/Guyana",
	"BOT":       "America/La_Paz",
	"BST":       "GB",
	"PET":       "America/Lima",
	"PEST":      "America/Lima",
	"PMST":      "America/Miquelon",
	"PMDT":      "America/Miquelon",
	"UYT":       "America/Montevideo",
	"UYST":      "America/Montevideo",
	"FNT":       "Brazil/DeNoronha",
	"FNST":      "Brazil/DeNoronha",
	"SRT":       "America/Paramaribo",
	"CLT":       "Chile/Continental",
	"CLST":      "Chile/Continental",
	"EHDT":      "America/Santo_Domingo",
	"EGT":       "America/Scoresbysund",
	"EGST":      "America/Scoresbysund",
	"NT":        "Canada/Newfoundland",
	"NST":       "Canada/Newfoundland",
	"NDT":       "Canada/Newfoundland",
	"AWT":       "Australia/West",
	"AWST":      "Australia/West",
	"NZT":       "NZ",
	"NZST":      "NZ",
	"NZDT":      "NZ",
	"ALMT":      "Asia/Almaty",
	"ALMST":     "Asia/Almaty",
	"ANAT":      "Asia/Anadyr",
	"AQTT":      "Asia/Aqtau",
	"AQTST":     "Asia/Aqtobe",
	"TMT":       "Asia/Ashgabat",
	"AZT":       "Asia/Baku",
	"AZST":      "Asia/Baku",
	"ICT":       "Asia/Bangkok",
	"KRAT":      "Asia/Krasnoyarsk",
	"KGT":       "Asia/Bishkek",
	"BNT":       "Asia/Brunei",
	"IST":       "Israel",
	"YAKT":      "Asia/Yakutsk",
	"YAKST":     "Asia/Yakutsk",
	"CHOT":      "Asia/Choibalsan",
	"CHOST":     "Asia/Choibalsan",
	"BDT":       "Asia/Dhaka",
	"BDST":      "Asia/Dhaka",
	"TLT":       "Asia/Dili",
	"GST":       "Atlantic/South_Georgia",
	"TJT":       "Asia/Dushanbe",
	"TSD":       "Asia/Dushanbe",
	"HKT":       "Asia/Hong_Kong",
	"HKST":      "Asia/Hong_Kong",
	"HOVT":      "Asia/Hovd",
	"HOVST":     "Asia/Hovd",
	"IRKT":      "Asia/Irkutsk",
	"IRKST":     "Asia/Irkutsk",
	"TRT":       "Turkey",
	"WIB":       "Asia/Jakarta",
	"WIT":       "Asia/Jayapura",
	"IDT":       "Israel",
	"AFT":       "Asia/Kabul",
	"PETT":      "Asia/Kamchatka",
	"PKT":       "Asia/Karachi",
	"PKST":      "Asia/Karachi",
	"NPT":       "Asia/Katmandu",
	"KRAST":     "Asia/Krasnoyarsk",
	"MYT":       "Asia/Kuala_Lumpur",
	"MLAST":     "Asia/Kuala_Lumpur",
	"BORTST":    "Asia/Kuching",
	"MAGT":      "Asia/Magadan",
	"MAGST":     "Asia/Magadan",
	"WITA":      "Asia/Makassar",
	"PHT":       "Asia/Manila",
	"PHST":      "Asia/Manila",
	"NOVT":      "Asia/Novosibirsk",
	"OMST":      "Asia/Omsk",
	"OMSST":     "Asia/Omsk",
	"ORAT":      "Asia/Oral",
	"KT":        "Asia/Seoul",
	"KST":       "Asia/Seoul",
	"QYZT":      "Asia/Qyzylorda",
	"QYZST":     "Asia/Qyzylorda",
	"MMT":       "Asia/Rangoon",
	"SAKT":      "Asia/Sakhalin",
	"UZT":       "Asia/Samarkand",
	"UZST":      "Asia/Samarkand",
	"KDT":       "Asia/Seoul",
	"SGT":       "Asia/Singapore",
	"MALST":     "Asia/Singapore",
	"SRET":      "Asia/Srednekolymsk",
	"GET":       "Asia/Tbilisi",
	"IRST":      "Iran",
	"IRDT":      "Iran",
	"BTT":       "Asia/Thimbu",
	"JST":       "Japan",
	"JDT":       "Japan",
	"ULAT":      "Asia/Ulaanbaatar",
	"ULAST":     "Asia/Ulaanbaatar",
	"VLAT":      "Asia/Vladivostok",
	"VLAST":     "Asia/Vladivostok",
	"YEKT":      "Asia/Yekaterinburg",
	"YEKST":     "Asia/Yekaterinburg",
	"AZOT":      "Atlantic/Azores",
	"AZOST":     "Atlantic/Azores",
	"CVT":       "Atlantic/Cape_Verde",
	"FKT":       "Atlantic/Stanley",
	"AET":       "Australia/ACT",
	"AEST":      "Australia/ACT",
	"AEDT":      "Australia/ACT",
	"ACDT":      "Australia/South",
	"ACWT":      "Australia/Eucla",
	"ACWST":     "Australia/Eucla",
	"ACWDT":     "Australia/Eucla",
	"LHT":       "Australia/LHI",
	"LHST":      "Australia/LHI",
	"LHDT":      "Australia/LHI",
	"AWDT":      "Australia/West",
	"EAST":      "Pacific/Easter",
	"EASST":     "Pacific/Easter",
	"UTC":       "UTC",
	"SAMT":      "Europe/Samara",
	"MSK":       "Europe/Moscow",
	"MSD":       "Europe/Moscow",
	"GMT+04:00": "Europe/Saratov",
	"VOLT":      "Europe/Volgograd",
	"IOT":       "Indian/Chagos",
	"CXT":       "Indian/Christmas",
	"CCT":       "Indian/Cocos",
	"TFT":       "Indian/Kerguelen",
	"SCT":       "Indian/Mahe",
	"MVT":       "Indian/Maldives",
	"MUT":       "Indian/Mauritius",
	"MUST":      "Indian/Mauritius",
	"RET":       "Indian/Reunion",
	"IRT":       "Iran",
	"MHT":       "Pacific/Majuro",
	"MET":       "MET",
	"MEST":      "MET",
	"CHAT":      "Pacific/Chatham",
	"CHAST":     "Pacific/Chatham",
	"CHADT":     "Pacific/Apia",
	"WSDT":      "Pacific/Apia",
	"CHUT":      "Pacific/Chuuk",
	"VUT":       "Pacific/Efate",
	"VUST":      "Pacific/Efate",
	"PHOT":      "Pacific/Enderbury",
	"TKT":       "Pacific/Fakaofo",
	"FJT":       "Pacific/Fiji",
	"FJST":      "Pacific/Fiji",
	"TVT":       "Pacific/Funafuti",
	"GALT":      "Pacific/Galapagos",
	"GAMT":      "Pacific/Gambier",
	"SBT":       "Pacific/Guadalcanal",
	"ChST":      "Pacific/Guam",
	"GDT":       "Pacific/Guam",
	"LINT":      "Pacific/Kiritimati",
	"KOST":      "Pacific/Kosrae",
	"MART":      "Pacific/Marquesas",
	"SST":       "Pacific/Samoa",
	"NRT":       "Pacific/Nauru",
	"NUT":       "Pacific/Niue",
	"NFT":       "Pacific/Norfolk",
	"NFDT":      "Pacific/Norfolk",
	"NCT":       "Pacific/Noumea",
	"NCST":      "Pacific/Noumea",
	"PWT":       "Pacific/Palau",
	"PONT":      "Pacific/Pohnpei",
	"PGT":       "Pacific/Port_Moresby",
	"CKT":       "Pacific/Rarotonga",
	"CKHST":     "Pacific/Rarotonga",
	"TAHT":      "Pacific/Tahiti",
	"GILT":      "Pacific/Tarawa",
	"TOT":       "Pacific/Tongatapu",
	"TOST":      "Pacific/Tongatapu",
	"WAKT":      "Pacific/Wake",
	"WFT":       "Pacific/Wallis",
}
