#! /bin/bash

useage='
Useage:
for single:  ./install.sh --singleton --unqiue_id xxx --instance_id xxx --ak xxx --sk xxx [--host xxx] [--port 9100]
for clusters:  ./install.sh --instance_id xxx [--host xxx] [--port 9100]
'

proc="node"
install_dir="/usr/local/cloudcare"
binary=${install_dir}/${proc}

if ! mkdir -p "${install_dir}" >/dev/null 2>&1; then
    echo "fail to create install dir!"
    exit 1
fi

download_url="http://cloudcare-carrier.oss-cn-hangzhou.aliyuncs.com/node_exporter"

unique_id=
instance_id=
ak=
sk=
host=
port=9100
remote_host=

singleton=0

upgrade=0

while [[ $# > 0 ]]
do
    opt=$1
    case "$opt" in

        --singleton)
            singleton=1
            shift
            continue
            ;;

        --upgrade)
            upgrade=1
            shift
            continue
            ;;

    esac

    if [ $# -lt 2 ]; then
        shift
        continue
    fi

    case "$opt" in

        --unique_id)
            unique_id="$2"
            shift
            ;;

        --instance_id)
            instance_id="$2"
            shift
            ;;

        --ak)
            ak="$2"
            shift
            ;;

        --sk)
            sk="$2"
            shift
            ;;

        --host)
            host="$2"
            shift
            ;;

        --remote_host)
            remote_host="$2"
            shift
            ;;

        --port)
            port="$2"
            shift
            ;;

        *)
            echo "unknown option \"$opt\""
            ;;
            
    esac
    shift
done

#check arguments
if [ $upgrade -eq 0 ]; then
    if [ $singleton -eq 1 ]; then
        if [ "$unique_id" = "" ] \
        || [ "$instance_id" = "" ] \
        || [ "$ak" = "" ] \
        || [ "$sk" = "" ]; then
            echo "$useage"
            exit 1
        fi
    else
        if [ "$instance_id" = "" ]; then
            echo "$useage"
            exit 1
        fi
    fi
fi

echo "start downloading..."
wget -O ${binary} ${download_url}
if [ $? -ne 0 ]; then
    exit 1
fi

chmod +x ${binary} >/dev/null 2>&1

retval=0

if [ $upgrade -eq 0 ]; then
    if [ $singleton -eq 0 ]; then
        ${binary} --init \
            --instance_id "${instance_id}" \
            --port "${port}" \
            ${host:+$(echo -n "--host ${host}")}

    else
        ${binary} --init \
            --singleton \
            --unique_id "${unique_id}" \
            --instance_id "${instance_id}" \
            --ak "${ak}" \
            --sk "${sk}" \
            --port "${port}" \
            ${host:+$(echo -n "--host ${host}")}
    fi

    retval=$?
fi

if [ $retval -eq 0 ]; then
    echo "ok"
else
    exit 1
fi

if [ $singleton -eq 1 ]; then
    ${binary} --singleton \
        ${remote_host:+$(echo -n "--remotehost ${remote_host}")}

fi
    ${binary}
fi

