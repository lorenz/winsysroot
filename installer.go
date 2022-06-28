package main

type Package struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Type     string `json:"type"`
	Payloads []struct {
		FileName string `json:"fileName"`
		Sha256   string `json:"sha256"`
		Size     int    `json:"size"`
		URL      string `json:"url"`
		Signer   struct {
			Ref string `json:"$ref"`
		} `json:"signer,omitempty"`
	} `json:"payloads,omitempty"`
	Dependencies map[string]interface{}
	InstallSizes struct {
		TargetDrive int `json:"targetDrive"`
	} `json:"installSizes,omitempty"`
}

type InstallerManifest struct {
	ManifestVersion string `json:"manifestVersion"`
	EngineVersion   string `json:"engineVersion"`
	Info            struct {
		ID                               string `json:"id"`
		BuildBranch                      string `json:"buildBranch"`
		BuildVersion                     string `json:"buildVersion"`
		LocalBuild                       string `json:"localBuild"`
		ManifestName                     string `json:"manifestName"`
		ManifestType                     string `json:"manifestType"`
		ProductDisplayVersion            string `json:"productDisplayVersion"`
		ProductLine                      string `json:"productLine"`
		ProductLineVersion               string `json:"productLineVersion"`
		ProductMilestone                 string `json:"productMilestone"`
		ProductMilestoneIsPreRelease     string `json:"productMilestoneIsPreRelease"`
		ProductName                      string `json:"productName"`
		ProductPatchVersion              string `json:"productPatchVersion"`
		ProductPreReleaseMilestoneSuffix string `json:"productPreReleaseMilestoneSuffix"`
		ProductSemanticVersion           string `json:"productSemanticVersion"`
	} `json:"info"`
	Signers []struct {
		ID          string `json:"$id"`
		SubjectName string `json:"subjectName"`
	} `json:"signers"`
	Packages  []Package `json:"packages"`
	Deprecate struct {
		ComponentMicrosoftVisualStudioTaskStatusCenter            string `json:"Component.Microsoft.VisualStudio.TaskStatusCenter"`
		ComponentMicrosoftVisualStudioASALExtensionOOB            string `json:"Component.Microsoft.VisualStudio.ASALExtensionOOB"`
		ComponentMicrosoftVisualStudioLanguageServerClientPreview string `json:"Component.Microsoft.VisualStudio.LanguageServer.Client.Preview"`
	} `json:"deprecate"`
	Signature struct {
		SignInfo struct {
			SignatureMethod  string `json:"signatureMethod"`
			DigestMethod     string `json:"digestMethod"`
			DigestValue      string `json:"digestValue"`
			Canonicalization string `json:"canonicalization"`
		} `json:"signInfo"`
		SignatureValue string `json:"signatureValue"`
		KeyInfo        struct {
			KeyValue struct {
				RsaKeyValue struct {
					Modulus  string `json:"modulus"`
					Exponent string `json:"exponent"`
				} `json:"rsaKeyValue"`
			} `json:"keyValue"`
			X509Data []string `json:"x509Data"`
		} `json:"keyInfo"`
		CounterSign struct {
			X509Data               []string `json:"x509Data"`
			Timestamp              string   `json:"timestamp"`
			CounterSignatureMethod string   `json:"counterSignatureMethod"`
			CounterSignature       string   `json:"counterSignature"`
		} `json:"counterSign"`
	} `json:"signature"`
}
