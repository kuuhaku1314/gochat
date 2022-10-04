package main

func AssertNotError(err error) {
	if err != nil {
		panic(err)
	}
}
