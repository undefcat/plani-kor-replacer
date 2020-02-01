package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	sc *bufio.Scanner
)

func init() {
	sc = bufio.NewScanner(os.Stdin)
}

func scanString() string {
	sc.Scan()
	return sc.Text()
}

func main() {
	fmt.Println("한글 추출 및 변환기")
	fmt.Println("1. 루트 디렉터리를 선택한다.")
	fmt.Println("2. 출력 디렉터리를 선택한다.")
	fmt.Println("3. 번역하지 않을 파일 확장자를 적는다.")
	fmt.Println("----------")

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	var rdPath string

	for {
		fmt.Printf("읽을 경로를 입력하세요. 현재경로: %s\n", wd)
		fmt.Printf("경로: ")

		// 루트 디렉터리
		rdPath, err = filepath.Abs(scanString())
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		// 디렉터리 존재유무 확인
		f, err := os.Stat(rdPath)
		if err != nil {
			switch {
			case os.IsNotExist(err):
				fmt.Printf("%s 경로는 존재하지 않습니다.\n", rdPath)

			default:
				fmt.Printf("%s", err.Error())
			}

			continue
		}

		// 디렉터리 여부 확인
		if ! f.IsDir() {
			fmt.Printf("%s 경로는 디렉터리가 아닙니다.\n", rdPath)
		}

		break
	}

	var outPath string

	for {
		fmt.Printf("출력 경로를 입력하세요. 현재경로: %s\n", wd)
		fmt.Printf("경로: ")

		// 출력 디렉터리
		outPath, err = filepath.Abs(scanString())
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		_, err := os.Stat(outPath)
		if err == nil {
			fmt.Printf("%s 경로는 이미 존재하는 경로입니다. 다른 경로를 선택해주세요.\n", outPath)
			continue
		}

		// 디렉터리 없는 경우 생성하고
		// 다음 단계로 넘어간다.
		if os.IsNotExist(err) {
			err := os.Mkdir(outPath, os.ModePerm)
			if err == nil {
				break
			}

			fmt.Println(err.Error())
			continue
		}

		break
	}

	fmt.Println("변환하지 않을 파일 확장자를 ,로 구분해서 공백 없이 적어주세요")
	fmt.Print("확장자(.를 포함해서 적어주세요): ")

	excludeExts := strings.Split(scanString(), ",")

	const PlaceHolder = "__PLACEHOLDER__"

	placeHolder := regexp.MustCompile(PlaceHolder)
	hidden := regexp.MustCompile(`__\((.+?)\)`)
	target := regexp.MustCompile(`[가-힣]+`)

	fmt.Println("작업을 시작합니다.")

	countChan := make(chan struct{}, 3)

	var wait sync.WaitGroup

	wait.Add(1)
	go func() {
		// 루트 디렉터리부터 순회
		err = filepath.Walk(rdPath, func(path string, info os.FileInfo, err error) error {

			if info.IsDir() {
				// continue
				return err
			}

			// 제외되는 확장자 확인
			currentExt := filepath.Ext(path)
			for _, ext := range excludeExts {
				if currentExt == ext {
					// continue
					return err
				}
			}

			// 파일을 읽는다.
			strs, err1 := ioutil.ReadFile(path)
			if err1 != nil {
				log.Println(err1)
				return err1
			}


			// 이미 존재하는 __() 함수 안의 한글들을 또 __()로 감싸면 안되므로
			// 우선 잠시 PlaceHolder로 대체한다.
			str := string(strs)
			placeholders := make([]string, 0, 16)
			str = hidden.ReplaceAllStringFunc(str, func(s string) string {
				placeholders = append(placeholders, s)

				return PlaceHolder
			})

			// 한글을 모두 찾고 __()로 감싼다.
			kors := make([]string, 0, 16)
			str = target.ReplaceAllStringFunc(str, func(s string) string {
				kors = append(kors, s)

				return fmt.Sprintf("__('%s')", s)
			})

			// 대체했던 PlaceHolder들을 원복시킨다.
			str = placeHolder.ReplaceAllStringFunc(str, func(s string) string {
				ret := placeholders[0]
				placeholders = placeholders[1:]

				return ret
			})

			// 출력경로
			outputPath := strings.Replace(path, rdPath, outPath, -1)
			outputDir := filepath.Dir(outputPath)

			// 존재하지 않으면 새로운 디렉터리를 생성한다.
			_, err = os.Stat(outputDir)
			if err != nil {
				if os.IsNotExist(err) {
					err1 := os.MkdirAll(outputDir, os.ModePerm)
					if err1 != nil {
						fmt.Println(err.Error())
						return err
					}

				} else {
					fmt.Println(err.Error())
					return err
				}
			}

			// 파일 생성
			f, err := os.Create(outputPath)
			defer f.Close()

			if err != nil {
				fmt.Println(err)
			}

			_, err = f.WriteString(str)
			if err != nil {
				fmt.Println(err)
			}

			// 완료된 파일작업 진행 현황을 알린다.
			countChan <- struct{}{}

			return err
		})

		if err != nil {
			fmt.Println(err.Error())
		}

		wait.Done()
		close(countChan)
	}()

	wait.Add(1)
	go func() {
		total := 0
		for range countChan {
			total++
			fmt.Printf("\r현재 %v번째 파일 작업을 완료했습니다.", total)
		}

		fmt.Printf("\r총 %v개의 파일을 작업 완료했습니다.", total)
		wait.Done()
	}()

	wait.Wait()
}