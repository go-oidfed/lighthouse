package version

import (
	_ "embed" // for go:embed
	"strconv"
	"strings"
)

// VERSION holds the server's version
//
//go:embed VERSION
var VERSION string

// Version segments
var (
	MAJOR int
	MINOR int
	FIX   int
	PRE   int
)

func init() {
	if VERSION[len(VERSION)-1] == '\n' {
		VERSION = VERSION[:len(VERSION)-1]
	}
	v := strings.Split(VERSION, ".")
	MAJOR, _ = strconv.Atoi(v[0])
	MINOR, _ = strconv.Atoi(v[1])
	ps := strings.Split(v[2], "-")
	FIX, _ = strconv.Atoi(ps[0])
	if len(ps) > 1 {
		pre := strings.TrimPrefix(ps[1], "pr")
		PRE, _ = strconv.Atoi(pre)
	}
}

var bannerMap = map[int32]string{
	'0': `
  ###   
 #   #  
#     # 
#     # 
#     # 
 #   #  
  ###   
`,
	'1': `
  #   
 ##   
# #   
  #   
  #   
  #   
##### 
`,
	'2': `
 #####  
#     # 
      # 
 #####  
#       
#       
####### 
        
`,
	'3': `
 #####  
#     # 
      # 
 #####  
      # 
#     # 
 #####  
`,
	'4': `
#       
#    #  
#    #  
#    #  
####### 
     #  
     #  
`,
	'5': `
####### 
#       
#       
######  
      # 
#     # 
 #####  
`,
	'6': `
 #####  
#     # 
#       
######  
#     # 
#     # 
 #####  
`,
	'7': `
####### 
#    #  
    #   
   #    
  #     
  #     
  #     
`,
	'8': `
 #####  
#     # 
#     # 
 #####  
#     # 
#     # 
 #####  
`,
	'9': `
 #####  
#     # 
#     # 
 ###### 
      # 
#     # 
 #####  
`,
	'.': `
    
    
    
    
### 
### 
### 
`,
}

func Banner(width int) string {
	// Build a banner for VERSION using bannerMap glyphs.
	// Characters not in bannerMap are ignored.

	// Helper to parse a glyph into lines and determine its width.
	parseGlyph := func(r rune) ([]string, int) {
		glyph, ok := bannerMap[int32(r)]
		if !ok {
			return nil, 0
		}
		// Remove leading/trailing newlines added by literal formatting
		glyph = strings.TrimPrefix(glyph, "\n")
		glyph = strings.TrimSuffix(glyph, "\n")
		lines := strings.Split(glyph, "\n")
		w := 0
		for _, l := range lines {
			if len(l) > w {
				w = len(l)
			}
		}
		return lines, w
	}

	// If width <= 0, treat as unlimited width
	maxWidth := width
	if maxWidth <= 0 {
		maxWidth = int(^uint(0) >> 1) // effectively infinite
	}

	var out strings.Builder

	var curLines []string
	curHeight := 0

	for _, r := range VERSION {
		lines, gWidth := parseGlyph(r)
		if lines == nil {
			// Ignore unsupported characters
			continue
		}

		if len(curLines) == 0 {
			// Start new group
			curLines = append([]string(nil), lines...)
			curHeight = len(lines)
			continue
		}

		// Attempt to append glyph to current group, with a single space as gap
		newHeight := curHeight
		if len(lines) > newHeight {
			newHeight = len(lines)
		}

		// Compute max current line width for padding if we need extra lines
		maxCurWidth := 0
		for _, l := range curLines {
			if len(l) > maxCurWidth {
				maxCurWidth = len(l)
			}
		}

		// Build tentative combined lines
		tentLines := make([]string, newHeight)
		maxTentWidth := 0
		for i := 0; i < newHeight; i++ {
			left := ""
			if i < curHeight {
				left = curLines[i]
			} else {
				left = ""
			}
			// Pad left to maxCurWidth to keep columns aligned
			if len(left) < maxCurWidth {
				left = left + strings.Repeat(" ", maxCurWidth-len(left))
			}
			right := ""
			if i < len(lines) {
				right = lines[i]
			} else {
				right = strings.Repeat(" ", gWidth)
			}
			combined := left + " " + right
			tentLines[i] = combined
			if len(combined) > maxTentWidth {
				maxTentWidth = len(combined)
			}
		}

		// Check width; if exceeded, flush current group and start a new one
		if maxTentWidth > maxWidth {
			// Flush current group, centered within the provided width
			if width > 0 {
				groupWidth := 0
				for _, l := range curLines {
					if len(l) > groupWidth {
						groupWidth = len(l)
					}
				}
				pad := 0
				if groupWidth < width {
					pad = (width - groupWidth) / 2
				}
				padStr := strings.Repeat(" ", pad)
				for i := 0; i < curHeight; i++ {
					out.WriteString(padStr)
					out.WriteString(curLines[i])
					out.WriteByte('\n')
				}
			} else {
				for i := 0; i < curHeight; i++ {
					out.WriteString(curLines[i])
					out.WriteByte('\n')
				}
			}
			// Start new group with current glyph
			curLines = append([]string(nil), lines...)
			curHeight = len(lines)
		} else {
			curLines = tentLines
			curHeight = newHeight
		}
	}

	// Flush any remaining lines
	if len(curLines) > 0 {
		if width > 0 {
			groupWidth := 0
			for _, l := range curLines {
				if len(l) > groupWidth {
					groupWidth = len(l)
				}
			}
			pad := 0
			if groupWidth < width {
				pad = (width - groupWidth) / 2
			}
			padStr := strings.Repeat(" ", pad)
			for i := 0; i < curHeight; i++ {
				out.WriteString(padStr)
				out.WriteString(curLines[i])
				out.WriteByte('\n')
			}
		} else {
			for i := 0; i < curHeight; i++ {
				out.WriteString(curLines[i])
				out.WriteByte('\n')
			}
		}
	}

	return out.String()
}
