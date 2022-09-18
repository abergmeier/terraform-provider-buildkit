package provider

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	portforwardtools "k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/cmd/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util"
)

var streams = genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

func newPortForwardOptions() *portforward.PortForwardOptions {
	return &portforward.PortForwardOptions{
		PortForwarder: &defaultPortForwarder{
			IOStreams: streams,
		},
	}
}

type portForwarder interface {
	ForwardPorts(method string, url *url.URL, opts portforward.PortForwardOptions) error
}

type defaultPortForwarder struct {
	genericclioptions.IOStreams
}

func (f *defaultPortForwarder) ForwardPorts(method string, url *url.URL, opts portforward.PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(opts.Config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	fw, err := portforwardtools.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

// splitPort splits port string which is in form of [LOCAL PORT]:REMOTE PORT
// and returns local and remote ports separately
func splitPort(port string) (local, remote string) {
	parts := strings.Split(port, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return parts[0], parts[0]
}

// Translates service port to target port
// It rewrites ports as needed if the Service port declares targetPort.
// It returns an error when a named targetPort can't find a match in the pod, or the Service did not declare
// the port.
func translateServicePortToTargetPort(ports []string, svc corev1.Service, pod corev1.Pod) ([]string, error) {
	var translated []string
	for _, port := range ports {
		localPort, remotePort := splitPort(port)

		portnum, err := strconv.Atoi(remotePort)
		if err != nil {
			svcPort, err := util.LookupServicePortNumberByName(svc, remotePort)
			if err != nil {
				return nil, err
			}
			portnum = int(svcPort)

			if localPort == remotePort {
				localPort = strconv.Itoa(portnum)
			}
		}
		containerPort, err := util.LookupContainerPortNumberByServicePort(svc, pod, int32(portnum))
		if err != nil {
			// can't resolve a named port, or Service did not declare this port, return an error
			return nil, err
		}

		// convert the resolved target port back to a string
		remotePort = strconv.Itoa(int(containerPort))

		if localPort != remotePort {
			translated = append(translated, fmt.Sprintf("%s:%s", localPort, remotePort))
		} else {
			translated = append(translated, remotePort)
		}
	}
	return translated, nil
}

// convertPodNamedPortToNumber converts named ports into port numbers
// It returns an error when a named port can't be found in the pod containers
func convertPodNamedPortToNumber(ports []string, pod corev1.Pod) ([]string, error) {
	var converted []string
	for _, port := range ports {
		localPort, remotePort := splitPort(port)

		containerPortStr := remotePort
		_, err := strconv.Atoi(remotePort)
		if err != nil {
			containerPort, err := util.LookupContainerPortNumberByName(pod, remotePort)
			if err != nil {
				return nil, err
			}

			containerPortStr = strconv.Itoa(int(containerPort))
		}

		if localPort != remotePort {
			converted = append(converted, fmt.Sprintf("%s:%s", localPort, containerPortStr))
		} else {
			converted = append(converted, containerPortStr)
		}
	}

	return converted, nil
}

func checkUDPPorts(udpOnlyPorts sets.Int, ports []string, obj metav1.Object) error {
	for _, port := range ports {
		_, remotePort := splitPort(port)
		portNum, err := strconv.Atoi(remotePort)
		if err != nil {
			switch v := obj.(type) {
			case *corev1.Service:
				svcPort, err := util.LookupServicePortNumberByName(*v, remotePort)
				if err != nil {
					return err
				}
				portNum = int(svcPort)

			case *corev1.Pod:
				ctPort, err := util.LookupContainerPortNumberByName(*v, remotePort)
				if err != nil {
					return err
				}
				portNum = int(ctPort)

			default:
				return fmt.Errorf("unknown object: %v", obj)
			}
		}
		if udpOnlyPorts.Has(portNum) {
			return fmt.Errorf("UDP protocol is not supported for %s", remotePort)
		}
	}
	return nil
}

// checkUDPPortInService returns an error if remote port in Service is a UDP port
// TODO: remove this check after #47862 is solved
func checkUDPPortInService(ports []string, svc *corev1.Service) error {
	udpPorts := sets.NewInt()
	tcpPorts := sets.NewInt()
	for _, port := range svc.Spec.Ports {
		portNum := int(port.Port)
		switch port.Protocol {
		case corev1.ProtocolUDP:
			udpPorts.Insert(portNum)
		case corev1.ProtocolTCP:
			tcpPorts.Insert(portNum)
		}
	}
	return checkUDPPorts(udpPorts.Difference(tcpPorts), ports, svc)
}

// checkUDPPortInPod returns an error if remote port in Pod is a UDP port
// TODO: remove this check after #47862 is solved
func checkUDPPortInPod(ports []string, pod *corev1.Pod) error {
	udpPorts := sets.NewInt()
	tcpPorts := sets.NewInt()
	for _, ct := range pod.Spec.Containers {
		for _, ctPort := range ct.Ports {
			portNum := int(ctPort.ContainerPort)
			switch ctPort.Protocol {
			case corev1.ProtocolUDP:
				udpPorts.Insert(portNum)
			case corev1.ProtocolTCP:
				tcpPorts.Insert(portNum)
			}
		}
	}
	return checkUDPPorts(udpPorts.Difference(tcpPorts), ports, pod)
}

type completionResult struct {
	obj            runtime.Object
	forwardablePod *corev1.Pod
}

func complete(f cmdutil.Factory, o *portforward.PortForwardOptions, resourceName string) (completionResult, error) {
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()

	builder := f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace()

	// TODO: Implement timeout
	/*
		getPodTimeout, err := cmdutil.GetPodRunningTimeoutFlag(cmd)
		if err != nil {
			return cmdutil.UsageErrorf(cmd, err.Error())
		}
	*/
	getPodTimeout := time.Minute

	builder.ResourceNames("pods", resourceName)

	res := completionResult{}
	res.obj, err = builder.Do().Object()
	if err != nil {
		return completionResult{}, err
	}

	res.forwardablePod, err = polymorphichelpers.AttachablePodForObjectFn(f, res.obj, getPodTimeout)
	if err != nil {
		return completionResult{}, err
	}

	o.PodName = res.forwardablePod.Name
	return res, nil
}

func completeService(f cmdutil.Factory, o *portforward.PortForwardOptions, resourceName string, ports []string) error {
	res, err := complete(f, o, resourceName)
	if err != nil {
		return err
	}
	// handle service port mapping to target port if needed
	t := res.obj.(*corev1.Service)
	err = checkUDPPortInService(ports, t)
	if err != nil {
		return err
	}
	o.Ports, err = translateServicePortToTargetPort(ports, *t, *res.forwardablePod)
	return err
}

func completePod(f cmdutil.Factory, o *portforward.PortForwardOptions, resourceName string, ports []string) error {
	res, err := complete(f, o, resourceName)
	err = checkUDPPortInPod(ports, res.forwardablePod)
	if err != nil {
		return err
	}
	o.Ports, err = convertPodNamedPortToNumber(ports, *res.forwardablePod)
	return err
}
