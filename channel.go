package main

type ChannelManifest struct {
	ManifestVersion string `json:"manifestVersion"`
	Info            struct {
		ID                               string `json:"id"`
		BuildBranch                      string `json:"buildBranch"`
		BuildVersion                     string `json:"buildVersion"`
		CommitID                         string `json:"commitId"`
		CommunityOrLowerFlightID         string `json:"communityOrLowerFlightId"`
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
		ProfessionalOrGreaterFlightID    string `json:"professionalOrGreaterFlightId"`
		QBuildSessionID                  string `json:"qBuildSessionId"`
	} `json:"info"`
	ChannelItems []struct {
		ID       string `json:"id"`
		Version  string `json:"version"`
		Type     string `json:"type"`
		Payloads []struct {
			FileName string `json:"fileName"`
			Sha256   string `json:"sha256"`
			Size     int    `json:"size"`
			URL      string `json:"url"`
		} `json:"payloads,omitempty"`
		Icon struct {
			MimeType string `json:"mimeType"`
			Base64   string `json:"base64"`
		} `json:"icon,omitempty"`
		IsHidden           bool   `json:"isHidden,omitempty"`
		ReleaseNotes       string `json:"releaseNotes,omitempty"`
		LocalizedResources []struct {
			Language    string `json:"language"`
			Title       string `json:"title"`
			Description string `json:"description"`
			License     string `json:"license"`
		} `json:"localizedResources,omitempty"`
		SupportsDownloadThenUpdate bool `json:"supportsDownloadThenUpdate,omitempty"`
		Requirements               struct {
			SupportedOS string `json:"supportedOS"`
			Conditions  struct {
				Expression string `json:"expression"`
				Conditions []struct {
					RegistryKey   string `json:"registryKey"`
					ID            string `json:"id"`
					RegistryValue string `json:"registryValue"`
					RegistryData  string `json:"registryData"`
				} `json:"conditions"`
			} `json:"conditions"`
		} `json:"requirements,omitempty"`
	} `json:"channelItems"`
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
