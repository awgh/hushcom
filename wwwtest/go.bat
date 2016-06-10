
md tmp\hushcom
rem md tmp\hushcom2
md tmp\hushcomd

copy /Y configs\hushcomd\ratnet.ql tmp\hushcomd\ratnet.ql

cd tmp\hushcomd
go build hushcom/hushcomd
start "HushCom Server" cmd /K hushcomd

cd ..\hushcom
xcopy /Y /E /I ..\..\js js
go build hushcom/hushcom
start "HushCom Client 1" cmd /K hushcom

rem cd ..\hushcom2
rem xcopy /Y /E /I ..\..\js js
rem go build hushcom/hushcom
rem start "HushCom Client 2" cmd /K hushcom  -p=20021 -ap=20022
