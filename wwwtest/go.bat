
md tmp\hushcom
md tmp\hushcom2
md tmp\hushcomd

copy /Y configs\hushcomd\ratnet.ql tmp\hushcomd\ratnet.ql

cd tmp\hushcomd
go build github.com/awgh/hushcom/hushcomd
start "HushCom Server" cmd /K hushcomd

cd ..\hushcom
xcopy /Y /E /I ..\..\js js
go build github.com/awgh/hushcom/hushcom
start "HushCom Client 1" cmd /K hushcom

cd ..\hushcom2
xcopy /Y /E /I ..\..\js js
go build github.com/awgh/hushcom/hushcom
start "HushCom Client 2" cmd /K hushcom  -p=20021
