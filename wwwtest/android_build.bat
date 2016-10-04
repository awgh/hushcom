md tmp
md tmp\android_client
cd tmp\android_client
set GOOS=android
set GOARCH=arm
set GOARM=7
set CGO_ENABLED=0
set NDK_TOOLCHAIN=C:\android-ndk\toolchains\arm-linux-androideabi-4.9\prebuilt\windows-x86_64\
set CC=%NDK_TOOLCHAIN%\bin\arm-linux-androideabi-gcc.exe

go build -v github.com/awgh/hushcom/hushcom
