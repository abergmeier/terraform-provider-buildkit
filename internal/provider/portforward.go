package provider

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubectl/pkg/cmd/portforward"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
)

func newPortForwardOptions() *portforward.PortForwardOptions {
	return &portforward.PortForwardOptions{
		PortForwarder: &defaultPortForwarder{
			IOStreams: streams,
		},
	}
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

func complete(o *portforward.PortForwardOptions, resourceName string) error {
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()

	builder := f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace()

	getPodTimeout, err := cmdutil.GetPodRunningTimeoutFlag(cmd)
	if err != nil {
		return cmdutil.UsageErrorf(cmd, err.Error())
	}

	builder.ResourceNames("pods", resourceName)

	obj, err := builder.Do().Object()
	if err != nil {
		return err
	}

	forwardablePod, err := polymorphichelpers.AttachablePodForObjectFn(f, obj, getPodTimeout)
	if err != nil {
		return err
	}

	o.PodName = forwardablePod.Name
}

func completeService(o *portforward.PortForwardOptions, resourceName string) error {
	err := complete(o, resourceName)
	if err != nil {
		return err
	}
	// handle service port mapping to target port if needed
	t := obj.(*corev1.Service)
	err = checkUDPPortInService(args[1:], t)
	if err != nil {
		return err
	}
	o.Ports, err = translateServicePortToTargetPort(args[1:], *t, *forwardablePod)
	if err != nil {
		return err
	}

	return nil
}

func completePod(o *portforward.PortForwardOptions, resourceName string) error {
	err := complete(o, resourceName)
	err = checkUDPPortInPod(args[1:], forwardablePod)
	if err != nil {
		return err
	}
	o.Ports, err = convertPodNamedPortToNumber(args[1:], *forwardablePod)
	if err != nil {
		return err
	}
}
