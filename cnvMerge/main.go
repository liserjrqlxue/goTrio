package main

import (
	"flag"
	"fmt"
	"github.com/biogo/biogo/util"
	"github.com/liserjrqlxue/simple-util"
	"github.com/tealeg/xlsx"
	"log"
	"os"
	"strconv"
	"strings"
)

type CNV struct {
	id         int
	chromosome string
	start, end int
	len        float64
	hitTag     int
	mergeTo    *CNV
	detail     string
	skip       bool
	rank       int
}

func newCNV(id, hitTag, rank int, bed []string) *CNV {
	chr, start, end := bed2region(bed)
	cnv := CNV{
		id:         id,
		chromosome: chr,
		start:      start,
		end:        end,
		len:        float64(end - start),
		hitTag:     hitTag,
		detail:     fmt.Sprintf("%03b\t%s", hitTag, strings.Join(bed, "\t")),
	}
	return &cnv
}

var (
	proband = flag.String(
		"100",
		"",
		"proband input bed",
	)
	father = flag.String(
		"010",
		"",
		"father input bed",
	)
	mother = flag.String(
		"001",
		"",
		"proband input bed",
	)
	overlapRate = flag.Float64(
		"overlapRate",
		0.8,
		"overlape rate",
	)
	prefix = flag.String(
		"prefix",
		"",
		"prefix of output",
	)
)

func main() {
	flag.Parse()
	if *proband == "" || *father == "" || *mother == "" || *prefix == "" {
		flag.Usage()
		os.Exit(1)
	}
	bed1Slice := simple_util.File2Slice(*proband, "\t")
	var CNVpool []*CNV
	for _, bed := range bed1Slice {
		cnv := newCNV(len(CNVpool), 0x1, 1, bed)
		CNVpool = append(CNVpool, cnv)
	}

	bed2Slice := simple_util.File2Slice(*father, "\t")
	for _, bed := range bed2Slice {
		cnv := newCNV(len(CNVpool), 0x2, 1, bed)
		CNVpool = append(CNVpool, cnv)
	}

	bed3Slice := simple_util.File2Slice(*mother, "\t")
	for _, bed := range bed3Slice {
		cnv := newCNV(len(CNVpool), 0x4, 1, bed)
		CNVpool = append(CNVpool, cnv)
	}

	var loop = true
	var len1, len2 int
	for loop {
		loop = false
		len2 = len(CNVpool)
		log.Printf("loop:%d->%d\n", len1, len2)
		for i := 0; i < len2; i++ {
			cnv1 := CNVpool[i]
			if cnv1.skip {
				continue
			}
			for j := util.Max(i+1, len1); j < len2; j++ {
				cnv2 := CNVpool[j]
				if cnv2.skip {
					continue
				}
				if checkOverlap(cnv1, cnv2) {
					cnv := mergeCNVs(cnv1, cnv2)
					cnv.id = len(CNVpool)
					CNVpool = append(CNVpool, cnv)
					loop = true
					break
				}
			}
			if cnv1.skip {
				continue
			}
		}
		len1 = len2
	}
	all, err := os.Create(*prefix + ".all.tsv")
	simple_util.CheckErr(err)
	lite, err := os.Create(*prefix + ".tsv")
	simple_util.CheckErr(err)
	allXlsx := xlsx.NewFile()
	allSheet, err := allXlsx.AddSheet("all")
	simple_util.CheckErr(err)
	liteSheet, err := allXlsx.AddSheet("lite")
	simple_util.CheckErr(err)
	allRow := allSheet.AddRow()
	liteRow := liteSheet.AddRow()
	for _, title := range []string{"ID", "Chromosome", "Start", "End", "HitTag", "Rank", "mergeTo", "Detail"} {
		allRow.AddCell().SetString(title)
		liteRow.AddCell().SetString(title)
	}
	for _, cnv := range CNVpool {
		if !cnv.skip {
			_, err = fmt.Fprintf(all, "%d\t%s\t%d\t%d\t%03b\t%d\t\t%s\n", cnv.id, cnv.chromosome, cnv.start, cnv.end, cnv.hitTag, cnv.rank, cnv.detail)
			simple_util.CheckErr(err)
			row := allSheet.AddRow()
			addCnvRow(cnv, row)
			_, err = fmt.Fprintf(lite, "%d\t%s\t%d\t%d\t%03b\t%d\t\t%s\n", cnv.id, cnv.chromosome, cnv.start, cnv.end, cnv.hitTag, cnv.rank, cnv.detail)
			simple_util.CheckErr(err)
			row = liteSheet.AddRow()
			addCnvRow(cnv, row)
		} else {
			_, err = fmt.Fprintf(all, "%d\t%s\t%d\t%d\t%03b\t%d\t%d\t%s\n", cnv.id, cnv.chromosome, cnv.start, cnv.end, cnv.hitTag, cnv.rank, cnv.mergeTo.id, cnv.detail)
			simple_util.CheckErr(err)
			row := allSheet.AddRow()
			addCnvRow(cnv, row)
		}
	}
	err = allXlsx.Save(*prefix + ".xlsx")
	simple_util.CheckErr(err)
}

func addCnvRow(cnv *CNV, row *xlsx.Row) {
	row.AddCell().SetInt(cnv.id)
	row.AddCell().SetString(cnv.chromosome)
	row.AddCell().SetInt(cnv.start)
	row.AddCell().SetInt(cnv.end)
	row.AddCell().SetString(fmt.Sprintf("%03b", cnv.hitTag))
	row.AddCell().SetInt(cnv.rank)
	if cnv.mergeTo != nil {
		row.AddCell().SetInt(cnv.mergeTo.id)
	} else {
		row.AddCell().SetString("")
	}

	row.AddCell().SetString(strings.Replace(cnv.detail, "<br>", "\t", -1))
}

func bed2region(bed []string) (chr string, start, end int) {
	if len(bed) < 3 {
		log.Fatalf("error format of bed:%+v", bed)
	}
	chr = bed[0]
	var err error
	start, err = strconv.Atoi(bed[1])
	simple_util.CheckErr(err)
	end, err = strconv.Atoi(bed[2])
	simple_util.CheckErr(err)
	return
}

func checkOverlap(cnv1, cnv2 *CNV) bool {
	if cnv1.chromosome == cnv2.chromosome {
		overlap := overlapLen(cnv1.start, cnv1.end, cnv2.start, cnv2.end)
		if overlap <= 0 {
			return false
		} else if overlap >= *overlapRate*cnv1.len || overlap >= *overlapRate*cnv2.len {
			return true
		}
	}
	return false
}

func mergeCNVs(cnv1, cnv2 *CNV) *CNV {
	cnv1.skip = true
	cnv2.skip = true
	chr := cnv1.chromosome
	start := util.Min(cnv1.start, cnv2.start)
	end := util.Max(cnv1.end, cnv2.end)
	hitTag := cnv1.hitTag | cnv2.hitTag
	rank := util.Max(cnv1.rank, cnv2.rank) + 1
	cnv := CNV{
		chromosome: chr,
		start:      start,
		end:        end,
		len:        float64(end - start),
		hitTag:     hitTag,
		rank:       rank,
		detail:     cnv1.detail + "<br>" + cnv2.detail,
	}
	cnv1.mergeTo = &cnv
	cnv2.mergeTo = &cnv
	return &cnv
}

func overlapLen(start1, end1, start2, end2 int) float64 {
	start := util.Max(start1, start2)
	end := util.Min(end1, end2)
	return float64(end - start)
}
